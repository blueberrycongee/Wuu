package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blueberrycongee/wuu/internal/securefs"
)

const pluginGenerationLeaseFile = ".plugin-generation.lock"

// PluginGenerationLease coordinates plugin execution and mutation across every
// app-server that shares a Wuu home. Executions hold shared leases; package or
// policy mutations hold the exclusive lease.
type PluginGenerationLease struct {
	mu        sync.Mutex
	file      *os.File
	exclusive bool
	epoch     uint64
}

func TryAcquirePluginGenerationExecutionLease(wuuHome string) (*PluginGenerationLease, bool, error) {
	return tryAcquirePluginGenerationLease(wuuHome, false)
}

func TryAcquirePluginGenerationMutationLease(wuuHome string) (*PluginGenerationLease, bool, error) {
	return tryAcquirePluginGenerationLease(wuuHome, true)
}

// ReadPluginGenerationEpoch reads the lightweight change signal without
// taking an execution lease. Watchers use it to avoid briefly blocking every
// package mutation while polling an unchanged generation. A concurrent write
// or transient read error is harmless to watchers: they retry on the next
// observation before acquiring the shared execution lease for refresh.
func ReadPluginGenerationEpoch(wuuHome string) (uint64, error) {
	wuuHome = strings.TrimSpace(wuuHome)
	if wuuHome == "" {
		return 0, errors.New("Wuu home is required")
	}
	file, err := os.Open(filepath.Join(wuuHome, pluginGenerationLeaseFile))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("open plugin generation epoch: %w", err)
	}
	defer file.Close()
	return readPluginGenerationEpoch(file)
}

func tryAcquirePluginGenerationLease(wuuHome string, exclusive bool) (*PluginGenerationLease, bool, error) {
	wuuHome = strings.TrimSpace(wuuHome)
	if wuuHome == "" {
		return nil, false, errors.New("Wuu home is required")
	}
	if err := securefs.Mkdir(wuuHome); err != nil {
		return nil, false, fmt.Errorf("create Wuu home for plugin generation lease: %w", err)
	}
	file, err := securefs.OpenFile(filepath.Join(wuuHome, pluginGenerationLeaseFile), os.O_CREATE|os.O_RDWR, securefs.FileMode)
	if err != nil {
		return nil, false, fmt.Errorf("open plugin generation lease: %w", err)
	}
	acquired, err := tryLockPluginGenerationFile(file, exclusive)
	if err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf("lock plugin generation lease: %w", err)
	}
	if !acquired {
		_ = file.Close()
		return nil, false, nil
	}
	epoch, err := readPluginGenerationEpoch(file)
	if err != nil {
		_ = unlockPluginGenerationFile(file)
		_ = file.Close()
		return nil, false, err
	}
	return &PluginGenerationLease{file: file, exclusive: exclusive, epoch: epoch}, true, nil
}

func (l *PluginGenerationLease) Epoch() uint64 {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.epoch
}

// Advance marks the start of a mutation generation while the exclusive lock is
// held. Advancing before disk changes is conservative: even a failed mutation
// makes peers revalidate rather than risk running a stale generation.
func (l *PluginGenerationLease) Advance() (uint64, error) {
	if l == nil {
		return 0, errors.New("plugin generation lease is required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil || !l.exclusive {
		return 0, errors.New("exclusive plugin generation lease is required")
	}
	l.epoch++
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], l.epoch)
	if _, err := l.file.WriteAt(encoded[:], 0); err != nil {
		return 0, fmt.Errorf("write plugin generation epoch: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return 0, fmt.Errorf("sync plugin generation epoch: %w", err)
	}
	return l.epoch, nil
}

func (l *PluginGenerationLease) Release() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	return errors.Join(unlockPluginGenerationFile(file), file.Close())
}

func readPluginGenerationEpoch(file *os.File) (uint64, error) {
	var encoded [8]byte
	n, err := file.ReadAt(encoded[:], 0)
	if errors.Is(err, io.EOF) && n == 0 {
		return 0, nil
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("read plugin generation epoch: %w", err)
	}
	if n != len(encoded) {
		return 0, errors.New("plugin generation epoch is corrupt")
	}
	return binary.BigEndian.Uint64(encoded[:]), nil
}
