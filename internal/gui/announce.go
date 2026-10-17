package gui

import (
	"odm/internal/engine"
)

// announcer tells the person about something that happened while ODM may
// be out of sight: a system notification and a sound. The app supplies it
// (tray.go); tests leave it nil.
type announcer interface {
	notify(title, body string)
	beep(failed bool)
}

func (s *Service) teller() announcer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.announce
}

// announceDone tells that a download finished or failed, as the Fyne UI
// does (internal/ui/notify_desktop.go): the system sound when Sounds is
// on, and a notification unless turned off. A finished download whose
// complete window just opened needs no notification: the window is in
// front.
func (s *Service) announceDone(d engine.Info, failed, windowShown bool) {
	a := s.teller()
	if a == nil {
		return
	}
	st := s.mgr.Settings()
	if st.Sounds {
		a.beep(failed)
	}
	if st.NoNotifications || (!failed && windowShown) {
		return
	}
	tr := s.translator()
	name := d.FileName
	if name == "" {
		name = d.URL
	}
	title, body := tr("Download complete"), name
	if failed {
		title = tr("Download failed")
		if d.Error != "" {
			body = name + ": " + d.Error
		}
	}
	a.notify(title, body)
}
