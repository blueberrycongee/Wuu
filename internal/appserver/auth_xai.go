package appserver

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/providers/xaisub"
)

func (s *Server) xaiLoginHub() *xaisub.LoginHub {
	s.xaiLoginMu.Lock()
	defer s.xaiLoginMu.Unlock()
	if s.xaiLogins == nil {
		s.xaiLogins = xaisub.NewLoginHub()
	}
	return s.xaiLogins
}

func (s *Server) handleAuthXAILoginStart(ctx context.Context, req Request) error {
	start, err := s.xaiLoginHub().Start(ctx)
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	return s.writeResponse(req.ID, AuthXAILoginStartResult{
		LoginID:                 start.LoginID,
		UserCode:                start.UserCode,
		VerificationURI:         start.VerificationURI,
		VerificationURIComplete: start.VerificationURIComplete,
		ExpiresInSeconds:        int(start.ExpiresIn.Seconds()),
		IntervalMS:              int(start.Interval.Milliseconds()),
	}, nil)
}

func (s *Server) handleAuthXAILoginPoll(ctx context.Context, req Request) error {
	var params AuthXAILoginPollParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	loginID := strings.TrimSpace(params.LoginID)
	if loginID == "" {
		return s.writeResponse(req.ID, nil, errLoginIDRequired())
	}
	status, err := s.xaiLoginHub().Poll(ctx, loginID, os.Getenv("HOME"), xaisub.DefaultBaseURL)
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	result := AuthXAILoginPollResult{
		Status:     status.Status,
		IntervalMS: int(status.Interval.Milliseconds()),
		Error:      status.Error,
	}
	if status.Status == xaisub.LoginSuccess {
		result.Providers = s.providerSummaries()
		s.resetXAISubscriptionRuntimes()
	}
	return s.writeResponse(req.ID, result, nil)
}

func (s *Server) handleAuthXAILoginCancel(req Request) error {
	var params AuthXAILoginCancelParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	s.xaiLoginHub().Cancel(strings.TrimSpace(params.LoginID))
	return s.writeResponse(req.ID, map[string]bool{"ok": true}, nil)
}

func (s *Server) resetXAISubscriptionRuntimes() {
	cfg, _, err := s.rt.LoadEffectiveConfig()
	if err != nil {
		return
	}
	if config.IsXAISubscriptionProvider(cfg.Providers[s.rt.ProviderName].Type) {
		s.resetThreadRuntimesForGeneralSettings("")
	}
}

func errLoginIDRequired() error {
	return errors.New("login_id is required")
}
