package session

import "testing"

func TestSessionOrganizationRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sess, err := CreateWithMetadata(dir, "organized-thread", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folder, err := CreateFolder(dir, "Topic")
	if err != nil {
		t.Fatal(err)
	}
	pinGroup, err := CreatePinGroup(dir, "Now")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := UpdateOrganization(dir, sess.ID, &folder.ID, &pinGroup.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.FolderID != folder.ID || updated.PinGroupID != pinGroup.ID || updated.PinnedAt == nil {
		t.Fatalf("organization not persisted: %+v", updated)
	}
	found, ok, err := Find(dir, sess.ID)
	if err != nil || !ok {
		t.Fatalf("find organized session: ok=%v err=%v", ok, err)
	}
	if found.FolderID != folder.ID || found.PinGroupID != pinGroup.ID {
		t.Fatalf("organization did not round-trip: %+v", found)
	}
	empty := ""
	updated, err = UpdateOrganization(dir, sess.ID, &empty, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.FolderID != "" || updated.PinGroupID != pinGroup.ID {
		t.Fatalf("folder-only update changed pin group: %+v", updated)
	}
	updated, err = UpdateOrganization(dir, sess.ID, &folder.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = UpdateOrganization(dir, sess.ID, nil, &empty)
	if err != nil {
		t.Fatal(err)
	}
	if updated.FolderID != folder.ID || updated.PinGroupID != "" || updated.PinnedAt != nil {
		t.Fatalf("pin-only update changed folder: %+v", updated)
	}
	organization, err := ListOrganization(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(organization.Folders) != 1 || len(organization.PinGroups) != 2 || !organization.PinGroups[0].Default {
		t.Fatalf("unexpected organization: %+v", organization)
	}
}

func TestOrganizationDeletionKeepsSessions(t *testing.T) {
	dir := t.TempDir()
	sess, err := CreateWithMetadata(dir, "organized-thread", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folder, _ := CreateFolder(dir, "Topic")
	pinGroup, _ := CreatePinGroup(dir, "Now")
	if _, err := UpdateOrganization(dir, sess.ID, &folder.ID, &pinGroup.ID); err != nil {
		t.Fatal(err)
	}
	if err := DeleteFolder(dir, folder.ID); err != nil {
		t.Fatal(err)
	}
	if err := DeletePinGroup(dir, pinGroup.ID); err != nil {
		t.Fatal(err)
	}
	found, ok, err := Find(dir, sess.ID)
	if err != nil || !ok {
		t.Fatalf("session was deleted with its groups: ok=%v err=%v", ok, err)
	}
	if found.FolderID != "" || found.PinGroupID != DefaultPinGroupID || found.PinnedAt == nil {
		t.Fatalf("unexpected deletion result: %+v", found)
	}
}

func TestBooleanPinUsesDefaultGroupAndArchiveClearsIt(t *testing.T) {
	dir := t.TempDir()
	sess, err := CreateWithMetadata(dir, "legacy-pin", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := UpdatePinned(dir, sess.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if pinned.PinGroupID != DefaultPinGroupID || pinned.PinnedAt == nil {
		t.Fatalf("legacy pin did not use default group: %+v", pinned)
	}
	archived, err := UpdateArchived(dir, sess.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if archived.PinGroupID != "" || archived.PinnedAt != nil {
		t.Fatalf("archive retained pin group: %+v", archived)
	}
}
