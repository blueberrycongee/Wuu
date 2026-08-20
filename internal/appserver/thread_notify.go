package appserver

import "github.com/blueberrycongee/wuu/internal/session"

func (s *Server) notifyThreadStarted(thread Thread) error {
	return s.writeNotification(NotificationThreadStarted, ThreadStartedNotification{Thread: thread})
}

func (s *Server) notifyThreadUpdated(thread Thread) error {
	if metadata, ok, err := session.Find(s.rt.SessionDir, thread.ID); err == nil && ok {
		thread.Pinned = metadata.PinnedAt != nil
		thread.FolderID = metadata.FolderID
		thread.PinGroupID = metadata.PinGroupID
	}
	return s.writeNotification(NotificationThreadUpdated, ThreadUpdatedNotification{Thread: thread})
}

func (s *Server) notifyOutboundBatch(batch []outboundNotification) {
	for _, item := range batch {
		if params, ok := item.params.(ThreadUpdatedNotification); ok {
			_ = s.notifyThreadUpdated(params.Thread)
			continue
		}
		_ = s.writeNotification(item.method, item.params)
	}
}
