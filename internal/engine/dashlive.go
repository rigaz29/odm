package engine

import (
	"context"
	"errors"
	"strconv"
	"time"

	"odm/internal/media"
)

// Live DASH recording.
//
// A dynamic MPD (type="dynamic") lists the segments that are available now
// and is reloaded while the broadcast runs: Facebook's and Instagram's
// players read one. ODM records it with the loop that records a live HLS
// playlist (live.go): the same state file, piece numbering, stop rules and
// assembly. Only reading the manifest differs.
//
// Gotchas:
//   - Segments are told apart, and ordered, by when they start (their
//     period's start plus their own, in milliseconds): their URLs may be
//     signed anew on every reload. That needs a SegmentTemplate, which
//     live manifests use; segments without a start all look alike.
//   - Init sections are told apart by period and Representation ID, not
//     URL: a fresh manifest from yt-dlp may put the same one elsewhere.
//   - A new recording starts at the live edge, with the last
//     dashLiveStart segments of each track, not the whole time-shift
//     buffer, which can be hours. One that continues after a gap takes
//     whatever the manifest still lists since.
//   - The tracks are the pinned Representations (VideoRep, AudioRep), or
//     the best of the last period. In a new period (an ad, a restart) a
//     track is found by ID, else as the best of its kind (pickRep).
//   - A manifest that turns static ends the recording once the rest is
//     fetched; one that is gone ends it at the stall limit.
//   - Templates counted from the clock need the computer's clock about
//     right (see media: one segment of margin); UTCTiming is not read.

// dashLiveStart is how many of each track's latest segments a new
// recording starts with, as a player starts near the live edge.
const dashLiveStart = 3

// recordDASHLive is recordLive for a live DASH manifest.
func (d *Download) recordDASHLive(ctx context.Context, job StreamJob) error {
	m, err := d.liveManifest(ctx, job.Manifest)
	if err != nil && job.Page != "" && ctx.Err() == nil {
		// A manifest found by yt-dlp may have expired since (a Resume
		// hours later): ask it for the broadcast's current one.
		if u, h, rerr := resolveSiteLive(ctx, job.Page, media.KindDASH); rerr == nil {
			d.useSiteManifest(u, h)
			job.Manifest = u
			m, err = d.liveManifest(ctx, job.Manifest)
		}
	}
	if err != nil {
		return err
	}
	video, audio := job.VideoRep, job.AudioRep
	if video == "" && audio == "" && len(m.Periods) > 0 {
		video, audio = BestDASH(&media.DASHManifest{Periods: m.Periods[len(m.Periods)-1:]})
		d.mu.Lock()
		if d.Stream.VideoRep == "" && d.Stream.AudioRep == "" {
			d.Stream.VideoRep, d.Stream.AudioRep = video, audio
		}
		d.mu.Unlock()
	}
	var ids []string
	for _, id := range []string{video, audio} {
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return refusal("the manifest lists no audio or video")
	}

	st, err := d.loadRecording(job)
	if err != nil {
		return err
	}
	if st == nil {
		st = &liveState{}
		for i := range ids {
			st.Tracks = append(st.Tracks, &liveTrack{Video: i == 0 && video != ""})
		}
	} else if len(st.Tracks) != len(ids) {
		return errors.New("the stream changed since this recording started; remove it and add it again")
	}
	r := newLiveRecorder(d, st, job)
	r.dash = ids
	return r.record(ctx)
}

// liveManifest reads a DASH manifest; one that cannot be parsed (an error
// page) is a refusal.
func (d *Download) liveManifest(ctx context.Context, manifest string) (*media.DASHManifest, error) {
	data, base, err := d.fetchText(ctx, manifest)
	if err != nil {
		return nil, err
	}
	m, err := media.ParseDASH(base, data)
	if err != nil {
		return nil, refusal(err.Error())
	}
	return m, nil
}

// dashLiveSeg is a segment of a track and the init section it needs.
type dashLiveSeg struct {
	seg    media.DASHSegment
	at     int64         // when it starts, in ms: its identity and order
	init   *media.HLSMap // nil when the Representation has none
	initID string        // the init's identity: period and Representation
}

// reloadDASH is reload for a DASH manifest: the new segments of every
// track, whether the manifest has ended (turned static), and how long to
// wait before the next reload (minimumUpdatePeriod, else a segment).
func (r *liveRecorder) reloadDASH(ctx context.Context) ([]pieceJob, bool, time.Duration, error) {
	r.lastTaken = nil
	m, err := r.d.liveManifest(ctx, r.job.Manifest)
	if err != nil {
		return nil, false, 0, err
	}
	var longest time.Duration
	r.inits = nil
	var jobs []pieceJob
	for ti, id := range r.dash {
		t := r.st.Tracks[ti]
		var segs []dashLiveSeg
		for _, p := range m.Periods {
			rep := pickRep(p, id, t.Video)
			if rep == nil {
				continue
			}
			if rep.Protected {
				return nil, false, 0, refusal("this stream is DRM-protected and cannot be downloaded")
			}
			if t.Ext == "" {
				t.Ext = dashExt(rep)
			}
			var init *media.HLSMap
			if rep.Init != nil {
				init = &media.HLSMap{URL: rep.Init.URL, Range: rep.Init.Range}
			}
			for _, s := range rep.Segments {
				segs = append(segs, dashLiveSeg{seg: s, at: (p.Start + s.Start).Milliseconds(), init: init, initID: p.ID + "/" + rep.ID})
				longest = max(longest, s.Duration)
			}
		}
		jobs = append(jobs, r.takeDASH(ti, segs)...)
	}
	r.lastTaken = jobs
	wait := m.UpdatePeriod
	if wait <= 0 {
		wait = longest
	}
	if wait <= 0 {
		wait = 2 * time.Second
	}
	return jobs, !m.Live, wait, nil
}

// takeDASH appends the track's new segments (and any init section they
// need) to track ti and returns them as jobs.
func (r *liveRecorder) takeDASH(ti int, segs []dashLiveSeg) []pieceJob {
	t := r.st.Tracks[ti]
	if len(segs) == 0 {
		return nil
	}
	if newest := segs[len(segs)-1].at; t.Started && newest <= t.LastSeq {
		if newest < t.LastSeq {
			r.regressed[ti]++
		} else {
			r.regressed[ti] = 0 // up to date, nothing new
		}
		// Times that stay behind for three reloads mean the stream
		// restarted: take its new times from the start.
		if r.regressed[ti] < 3 {
			r.noteDASHInits(ti, segs)
			return nil
		}
		t.LastSeq = segs[0].at - 1
	}
	r.regressed[ti] = 0
	if !t.Started && len(segs) > dashLiveStart {
		segs = segs[len(segs)-dashLiveStart:]
	}
	var jobs []pieceJob
	for _, s := range segs {
		if t.Started && s.at <= t.LastSeq {
			continue
		}
		idx := -1
		if s.init != nil {
			for i, have := range t.Inits {
				if have == s.initID {
					idx = i
					break
				}
			}
			if idx < 0 {
				idx = len(t.Inits)
				t.Inits = append(t.Inits, s.initID)
				jobs = append(jobs, initJob(ti, idx, s.init))
			}
		}
		n := len(t.SegInit)
		t.SegInit = append(t.SegInit, idx)
		jobs = append(jobs, pieceJob{track: ti, name: strconv.Itoa(n),
			p: &piece{url: s.seg.URL, rng: s.seg.Range, seq: s.at}})
		t.LastSeq, t.Started = s.at, true
	}
	r.noteDASHInits(ti, segs)
	return jobs
}

// noteDASHInits is noteInits for DASH: the current URL of each init piece
// the manifest still uses.
func (r *liveRecorder) noteDASHInits(ti int, segs []dashLiveSeg) {
	t := r.st.Tracks[ti]
	for _, s := range segs {
		if s.init == nil {
			continue
		}
		for i, have := range t.Inits {
			if have == s.initID {
				r.initURLs(ti)[i] = s.init
				break
			}
		}
	}
}

// resolveDASH asks yt-dlp for the broadcast's current manifest, for one it
// found on a page: its address is signed and expires.
func (r *liveRecorder) resolveDASH(ctx context.Context) {
	if r.job.Page == "" {
		return
	}
	if u, h, err := resolveSiteLive(ctx, r.job.Page, media.KindDASH); err == nil {
		r.d.useSiteManifest(u, h)
		r.job.Manifest = u
	}
}
