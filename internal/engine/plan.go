package engine

import "time"

// NewPlan splits [0,total) into at most conns chunks. total<=0 (unknown size)
// yields a single open-ended chunk that is filled sequentially.
func NewPlan(total int64, conns int) []*Chunk {
	if conns < 1 {
		conns = 1
	}
	if total <= 0 {
		return []*Chunk{{ID: 0, Start: 0, End: -1}}
	}
	size := (total + int64(conns) - 1) / int64(conns)
	if size < MinChunkSize {
		size = MinChunkSize
	}
	var chunks []*Chunk
	for start, id := int64(0), 0; start < total; id++ {
		end := start + size - 1
		if end >= total {
			end = total - 1
		}
		chunks = append(chunks, &Chunk{ID: id, Start: start, End: end})
		start = end + 1
	}
	// Fold a too-small tail into the previous chunk instead of scheduling a
	// tiny request, matching the minimum-chunk behaviour.
	if n := len(chunks); n > 1 && chunks[n-1].Size() < MinChunkSize {
		chunks[n-2].End = chunks[n-1].End
		chunks = chunks[:n-1]
	}
	return chunks
}

// A connection at least takeoverRatio times slower than a freed one hands it
// all it has left, when that is at least takeoverMin bytes it would still
// need longer than takeoverAfter for. Halving it, as Split does, kept the
// last piece on the slow connection while every other one had finished.
const (
	takeoverRatio = 4
	takeoverAfter = time.Second
	takeoverMin   = 16 << 10
)

// takeOver moves the rest of the slowest chunk that qualifies (see
// takeoverRatio) to a new chunk for a freed connection whose last pace was
// helperPace, or returns nil. The slow connection stops at the bytes it
// already counted; its chunk is then complete.
func takeOver(chunks []*Chunk, helperPace int64) *Chunk {
	if helperPace <= 0 {
		return nil
	}
	maxID := -1
	for _, c := range chunks {
		maxID = max(maxID, c.ID)
	}
	var victim *Chunk
	var worst float64
	for _, c := range chunks {
		if c.Complete() || c.End < 0 || c.pace <= 0 || c.Done == 0 {
			continue
		}
		r := c.Remaining()
		if r < takeoverMin || c.pace*takeoverRatio > helperPace {
			continue
		}
		eta := float64(r) / float64(c.pace)
		if eta < takeoverAfter.Seconds() {
			continue
		}
		if victim == nil || eta > worst {
			victim, worst = c, eta
		}
	}
	if victim == nil {
		return nil
	}
	at := victim.Next()
	nc := &Chunk{ID: maxID + 1, Start: at, End: victim.End}
	victim.End = at - 1
	return nc
}

// splitFor finds work for a freed connection whose last pace was
// helperPace (0 when not known): all of a far slower chunk's rest, else the
// tail half of the chunk Split picks. A connection far slower than that
// chunk's own gets nothing: half its work would finish later here.
func splitFor(chunks []*Chunk, minChunk, helperPace int64) (*Chunk, bool) {
	if nc := takeOver(chunks, helperPace); nc != nil {
		return nc, true
	}
	victim := splitVictim(chunks, minChunk)
	if victim == nil {
		return nil, false
	}
	if helperPace > 0 && victim.pace > 0 && helperPace*takeoverRatio <= victim.pace {
		return nil, false
	}
	return cutHalf(chunks, victim)
}

// Split carves the tail half off one incomplete chunk and returns it as a
// new chunk for a freed connection. The chunk that will take the longest to
// finish is chosen when two paces are known and the slower one will take at
// least twice as long. Otherwise the largest remainder is chosen.
// It reports false when no chunk is big enough to split safely.
func Split(chunks []*Chunk, minChunk int64) (*Chunk, bool) {
	victim := splitVictim(chunks, minChunk)
	if victim == nil {
		return nil, false
	}
	return cutHalf(chunks, victim)
}

// splitVictim is the chunk Split halves, or nil.
func splitVictim(chunks []*Chunk, minChunk int64) *Chunk {
	var largest, slow *Chunk
	var bestRemain int64
	var slowETA, fastETA float64
	paced := 0
	for _, c := range chunks {
		if c.Complete() {
			continue
		}
		r := c.Remaining()
		if r < 2*minChunk {
			continue
		}
		if r > bestRemain {
			bestRemain, largest = r, c
		}
		if c.pace <= 0 {
			continue
		}
		eta := float64(r) / float64(c.pace)
		paced++
		if slow == nil || eta > slowETA {
			slow, slowETA = c, eta
		}
		if paced == 1 || eta < fastETA {
			fastETA = eta
		}
	}
	victim := largest
	if paced >= 2 && slow != nil && fastETA > 0 && slowETA >= 2*fastETA {
		victim = slow
	}
	return victim
}

// cutHalf moves the tail half of victim's rest to a new chunk.
func cutHalf(chunks []*Chunk, victim *Chunk) (*Chunk, bool) {
	maxID := -1
	for _, c := range chunks {
		maxID = max(maxID, c.ID)
	}
	mid := victim.Next() + victim.Remaining()/2
	if mid <= victim.Start || mid > victim.End {
		return nil, false
	}
	nc := &Chunk{ID: maxID + 1, Start: mid, End: victim.End}
	victim.End = mid - 1
	return nc, true
}
