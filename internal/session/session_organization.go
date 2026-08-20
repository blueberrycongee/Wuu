package session

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultPinGroupID = "default"

type OrganizationGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	Default   bool      `json:"default,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionOrganization struct {
	Folders   []OrganizationGroup `json:"folders"`
	PinGroups []OrganizationGroup `json:"pin_groups"`
}

type organizationGroupKind string

const (
	groupKindFolder organizationGroupKind = "folder"
	groupKindPin    organizationGroupKind = "pin"
)

func groupTable(kind organizationGroupKind) (string, error) {
	switch kind {
	case groupKindFolder:
		return "session_folders", nil
	case groupKindPin:
		return "pin_groups", nil
	default:
		return "", fmt.Errorf("unknown organization group kind %q", kind)
	}
}

func ListOrganization(sessDir string) (SessionOrganization, error) {
	db, err := openStore(sessDir)
	if err != nil {
		return SessionOrganization{}, err
	}
	defer db.Close()
	folders, err := listOrganizationGroups(db, groupKindFolder)
	if err != nil {
		return SessionOrganization{}, err
	}
	pinGroups, err := listOrganizationGroups(db, groupKindPin)
	if err != nil {
		return SessionOrganization{}, err
	}
	return SessionOrganization{Folders: folders, PinGroups: pinGroups}, nil
}

func listOrganizationGroups(db *sql.DB, kind organizationGroupKind) ([]OrganizationGroup, error) {
	table, err := groupTable(kind)
	if err != nil {
		return nil, err
	}
	defaultColumn := "0"
	if kind == groupKindPin {
		defaultColumn = "is_default"
	}
	rows, err := db.Query(fmt.Sprintf(`SELECT id, name, sort_order, %s, created_at, updated_at FROM %s ORDER BY sort_order, created_at, id`, defaultColumn, table))
	if err != nil {
		return nil, fmt.Errorf("list %s groups: %w", kind, err)
	}
	defer rows.Close()
	groups := make([]OrganizationGroup, 0)
	for rows.Next() {
		var group OrganizationGroup
		var isDefault int
		var createdAt, updatedAt string
		if err := rows.Scan(&group.ID, &group.Name, &group.SortOrder, &isDefault, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan %s group: %w", kind, err)
		}
		group.Default = isDefault != 0
		group.CreatedAt = parseTime(createdAt)
		group.UpdatedAt = parseTime(updatedAt)
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func CreateFolder(sessDir, name string) (OrganizationGroup, error) {
	return createOrganizationGroup(sessDir, groupKindFolder, name)
}

func CreatePinGroup(sessDir, name string) (OrganizationGroup, error) {
	return createOrganizationGroup(sessDir, groupKindPin, name)
}

func createOrganizationGroup(sessDir string, kind organizationGroupKind, name string) (OrganizationGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return OrganizationGroup{}, errors.New("group name is required")
	}
	table, err := groupTable(kind)
	if err != nil {
		return OrganizationGroup{}, err
	}
	db, err := openStore(sessDir)
	if err != nil {
		return OrganizationGroup{}, err
	}
	defer db.Close()
	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	now := time.Now().UTC()
	group := OrganizationGroup{ID: NewID(), Name: name, CreatedAt: now, UpdatedAt: now}
	if err := db.QueryRow(fmt.Sprintf(`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM %s`, table)).Scan(&group.SortOrder); err != nil {
		return OrganizationGroup{}, fmt.Errorf("next %s group order: %w", kind, err)
	}
	if kind == groupKindPin {
		_, err = db.Exec(`INSERT INTO pin_groups (id, name, sort_order, is_default, created_at, updated_at) VALUES (?, ?, ?, 0, ?, ?)`, group.ID, group.Name, group.SortOrder, timeText(now), timeText(now))
	} else {
		_, err = db.Exec(`INSERT INTO session_folders (id, name, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, group.ID, group.Name, group.SortOrder, timeText(now), timeText(now))
	}
	if err != nil {
		return OrganizationGroup{}, fmt.Errorf("create %s group: %w", kind, err)
	}
	return group, nil
}

func RenameFolder(sessDir, id, name string) (OrganizationGroup, error) {
	return renameOrganizationGroup(sessDir, groupKindFolder, id, name)
}

func RenamePinGroup(sessDir, id, name string) (OrganizationGroup, error) {
	return renameOrganizationGroup(sessDir, groupKindPin, id, name)
}

func renameOrganizationGroup(sessDir string, kind organizationGroupKind, id, name string) (OrganizationGroup, error) {
	id, name = strings.TrimSpace(id), strings.TrimSpace(name)
	if id == "" || name == "" {
		return OrganizationGroup{}, errors.New("group id and name are required")
	}
	if kind == groupKindPin && id == DefaultPinGroupID {
		return OrganizationGroup{}, errors.New("the default pin group cannot be renamed")
	}
	table, err := groupTable(kind)
	if err != nil {
		return OrganizationGroup{}, err
	}
	db, err := openStore(sessDir)
	if err != nil {
		return OrganizationGroup{}, err
	}
	defer db.Close()
	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	now := time.Now().UTC()
	result, err := db.Exec(fmt.Sprintf(`UPDATE %s SET name = ?, updated_at = ? WHERE id = ?`, table), name, timeText(now), id)
	if err != nil {
		return OrganizationGroup{}, fmt.Errorf("rename %s group: %w", kind, err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return OrganizationGroup{}, fmt.Errorf("%s group not found: %q", kind, id)
	}
	var group OrganizationGroup
	var createdAt string
	if err := db.QueryRow(fmt.Sprintf(`SELECT id, name, sort_order, created_at FROM %s WHERE id = ?`, table), id).Scan(&group.ID, &group.Name, &group.SortOrder, &createdAt); err != nil {
		return OrganizationGroup{}, err
	}
	group.CreatedAt, group.UpdatedAt = parseTime(createdAt), now
	return group, nil
}

func DeleteFolder(sessDir, id string) error {
	return deleteOrganizationGroup(sessDir, groupKindFolder, id)
}

func DeletePinGroup(sessDir, id string) error {
	return deleteOrganizationGroup(sessDir, groupKindPin, id)
}

func ReorderFolders(sessDir string, ids []string) error {
	return reorderOrganizationGroups(sessDir, groupKindFolder, ids)
}

func ReorderPinGroups(sessDir string, ids []string) error {
	return reorderOrganizationGroups(sessDir, groupKindPin, ids)
}

func reorderOrganizationGroups(sessDir string, kind organizationGroupKind, ids []string) error {
	table, err := groupTable(kind)
	if err != nil {
		return err
	}
	db, err := openStore(sessDir)
	if err != nil {
		return err
	}
	defer db.Close()
	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query := fmt.Sprintf(`SELECT id FROM %s`, table)
	if kind == groupKindPin {
		query += ` WHERE id <> '` + DefaultPinGroupID + `'`
	}
	rows, err := tx.Query(query)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		existing[id] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(ids) != len(existing) {
		return errors.New("reorder must include every group exactly once")
	}
	now := time.Now().UTC()
	base := 0
	if kind == groupKindPin {
		base = 1
	}
	seen := make(map[string]struct{}, len(ids))
	for index, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if _, ok := existing[id]; !ok {
			return fmt.Errorf("%s group not found: %q", kind, id)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("duplicate %s group: %q", kind, id)
		}
		seen[id] = struct{}{}
		if _, err := tx.Exec(fmt.Sprintf(`UPDATE %s SET sort_order = ?, updated_at = ? WHERE id = ?`, table), base+index, timeText(now), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func deleteOrganizationGroup(sessDir string, kind organizationGroupKind, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("group id is required")
	}
	if kind == groupKindPin && id == DefaultPinGroupID {
		return errors.New("the default pin group cannot be deleted")
	}
	table, err := groupTable(kind)
	if err != nil {
		return err
	}
	db, err := openStore(sessDir)
	if err != nil {
		return err
	}
	defer db.Close()
	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if kind == groupKindFolder {
		if _, err := tx.Exec(`UPDATE sessions SET folder_id = '' WHERE folder_id = ?`, id); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(`UPDATE sessions SET pin_group_id = ? WHERE pin_group_id = ?`, DefaultPinGroupID, id); err != nil {
			return err
		}
	}
	result, err := tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, table), id)
	if err != nil {
		return fmt.Errorf("delete %s group: %w", kind, err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return fmt.Errorf("%s group not found: %q", kind, id)
	}
	return tx.Commit()
}

func UpdateOrganization(sessDir, id string, folderID, pinGroupID *string) (Session, error) {
	return updateSessionOrganization(sessDir, id, folderID, pinGroupID, nil)
}

func UpdatePinnedInGroup(sessDir, id string, pinned bool, pinGroupID string) (Session, error) {
	if pinned && strings.TrimSpace(pinGroupID) == "" {
		pinGroupID = DefaultPinGroupID
	}
	return updateSessionOrganization(sessDir, id, nil, &pinGroupID, &pinned)
}

func updateSessionOrganization(sessDir, id string, folderID, pinGroupID *string, pinned *bool) (Session, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Session{}, fmt.Errorf("%w: %q", ErrSessionNotFound, id)
	}
	db, err := openStore(sessDir)
	if err != nil {
		return Session{}, err
	}
	defer db.Close()
	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback()
	sess, ok, err := findSessionTx(tx, id)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		return Session{}, fmt.Errorf("%w: %q", ErrSessionNotFound, id)
	}
	if pinned == nil {
		if folderID != nil {
			value := strings.TrimSpace(*folderID)
			if err := validateGroupIDTx(tx, groupKindFolder, value); err != nil {
				return Session{}, err
			}
			sess.FolderID = value
		}
		if pinGroupID != nil {
			value := strings.TrimSpace(*pinGroupID)
			if err := validateGroupIDTx(tx, groupKindPin, value); err != nil {
				return Session{}, err
			}
			sess.PinGroupID = value
			if value == "" {
				sess.PinnedAt = nil
			} else if sess.PinnedAt == nil {
				now := time.Now().UTC()
				sess.PinnedAt = &now
			}
		}
	} else if *pinned {
		value := DefaultPinGroupID
		if pinGroupID != nil && strings.TrimSpace(*pinGroupID) != "" {
			value = strings.TrimSpace(*pinGroupID)
		}
		if err := validateGroupIDTx(tx, groupKindPin, value); err != nil {
			return Session{}, err
		}
		sess.PinGroupID = value
		now := time.Now().UTC()
		sess.PinnedAt = &now
	} else {
		sess.PinGroupID = ""
		sess.PinnedAt = nil
	}
	if err := updateSessionTx(tx, sess); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	return sess, nil
}

func validateGroupIDTx(tx *sql.Tx, kind organizationGroupKind, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	table, err := groupTable(kind)
	if err != nil {
		return err
	}
	var found string
	err = tx.QueryRow(fmt.Sprintf(`SELECT id FROM %s WHERE id = ?`, table), id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s group not found: %q", kind, id)
	}
	return err
}
