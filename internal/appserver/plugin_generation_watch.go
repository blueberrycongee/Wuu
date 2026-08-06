package appserver

import (
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/session"
)

const pluginGenerationWatchInterval = 100 * time.Millisecond

func (s *Server) startPluginGenerationWatch() {
	if s == nil || s.rt == nil || s.rt.WuuHome == "" || s.startupErr != nil {
		return
	}
	s.startBackground(func() {
		ticker := time.NewTicker(pluginGenerationWatchInterval)
		defer ticker.Stop()
		for !s.closed.Load() {
			<-ticker.C
			if err := s.refreshPluginGenerationIfChanged(); err != nil {
				providers.DebugLogf("refresh observed plugin generation: %v", err)
			}
		}
	})
}

func (s *Server) refreshPluginGenerationIfChanged() error {
	if s == nil || s.rt == nil || s.rt.WuuHome == "" || s.closed.Load() {
		return nil
	}
	lease, acquired, err := session.TryAcquirePluginGenerationExecutionLease(s.rt.WuuHome)
	if err != nil || !acquired {
		return err
	}
	defer lease.Release()
	epoch := lease.Epoch()
	if epoch == s.pluginGenerationEpoch.Load() {
		return nil
	}

	s.pluginGenerationRefreshMu.Lock()
	defer s.pluginGenerationRefreshMu.Unlock()
	if epoch == s.pluginGenerationEpoch.Load() {
		return nil
	}
	inventory, skills, err := s.refreshPluginPackages()
	if err != nil {
		return err
	}
	s.pluginGenerationEpoch.Store(epoch)
	return s.writeNotification(NotificationPluginInventoryChanged, PluginInventoryChangedNotification{
		Epoch:              epoch,
		ExtensionInventory: inventory,
		Skills:             skills,
	})
}
