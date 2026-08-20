package appserver

import (
	"errors"
	"strings"

	"github.com/blueberrycongee/wuu/internal/session"
)

func (s *Server) handleSessionOrganizationList(req Request) error {
	organization, err := session.ListOrganization(s.rt.SessionDir)
	return s.writeResponse(req.ID, SessionOrganizationListResult{Organization: organization}, err)
}

func (s *Server) handleOrganizationGroupCreate(req Request, pin bool) error {
	var params OrganizationGroupCreateParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	var group session.OrganizationGroup
	var err error
	if pin {
		group, err = session.CreatePinGroup(s.rt.SessionDir, params.Name)
	} else {
		group, err = session.CreateFolder(s.rt.SessionDir, params.Name)
	}
	return s.writeResponse(req.ID, OrganizationGroupResult{Group: group}, err)
}

func (s *Server) handleOrganizationGroupUpdate(req Request, pin bool) error {
	var params OrganizationGroupUpdateParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	var group session.OrganizationGroup
	var err error
	if pin {
		group, err = session.RenamePinGroup(s.rt.SessionDir, params.ID, params.Name)
	} else {
		group, err = session.RenameFolder(s.rt.SessionDir, params.ID, params.Name)
	}
	return s.writeResponse(req.ID, OrganizationGroupResult{Group: group}, err)
}

func (s *Server) handleOrganizationGroupDelete(req Request, pin bool) error {
	var params OrganizationGroupDeleteParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	sessions, err := session.List(s.rt.SessionDir, 0)
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	affected := make([]string, 0)
	for _, sess := range sessions {
		if (!pin && sess.FolderID == params.ID) || (pin && sess.PinGroupID == params.ID) {
			affected = append(affected, sess.ID)
		}
	}
	if pin {
		err = session.DeletePinGroup(s.rt.SessionDir, params.ID)
	} else {
		err = session.DeleteFolder(s.rt.SessionDir, params.ID)
	}
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	if err := s.writeResponse(req.ID, map[string]bool{"ok": true}, nil); err != nil {
		return err
	}
	for _, id := range affected {
		metadata, ok, findErr := session.Find(s.rt.SessionDir, id)
		if findErr != nil || !ok {
			continue
		}
		thread, updateErr := s.threadAfterMetadataUpdate(metadata)
		if updateErr == nil {
			_ = s.notifyThreadUpdated(thread)
		}
	}
	return nil
}

func (s *Server) handleThreadOrganizationUpdate(req Request) error {
	var params ThreadOrganizationUpdateParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	id := strings.TrimSpace(params.ThreadID)
	if id == "" {
		return s.writeResponse(req.ID, nil, errors.New("thread_id is required"))
	}
	metadata, err := session.UpdateOrganization(s.rt.SessionDir, id, params.FolderID, params.PinGroupID)
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	thread, err := s.threadAfterMetadataUpdate(metadata)
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	if err := s.writeResponse(req.ID, ThreadOrganizationUpdateResult{Thread: thread}, nil); err != nil {
		return err
	}
	return s.notifyThreadUpdated(thread)
}
