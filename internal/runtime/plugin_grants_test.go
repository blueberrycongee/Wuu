package runtime

import (
	"reflect"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/extensions"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
)

// Official bundled packages receive the full closed catalog explicitly;
// community packages receive exactly the granted set; anything unresolved is
// nil (fail closed at the plugin host).
func TestPluginGrantedPermissionsResolver(t *testing.T) {
	official := pluginpkg.Plugin{Official: true}
	community := pluginpkg.Plugin{
		Manifest:    pluginpkg.Manifest{ID: "community"},
		SubjectID:   "plugin:user:community",
		Fingerprint: "fp-1",
	}

	empty := pluginGrantedPermissions(config.Config{})
	if got := empty(community); got != nil {
		t.Fatalf("no settings community grant = %v, want nil", got)
	}

	settings := &extensions.Settings{
		Grants: map[string]extensions.Grant{
			"plugin:user:community": {
				SubjectID:   "plugin:user:community",
				Fingerprint: "fp-1",
				Scope:       extensions.GrantScopeUser,
				Permissions: []string{"process.spawn", "session.read"},
				ApprovedAt:  time.Now().UTC(),
			},
		},
	}
	resolve := pluginGrantedPermissions(config.Config{Extensions: settings})

	if got := resolve(official); !reflect.DeepEqual(got, extensions.CatalogPermissions()) {
		t.Fatalf("official grant = %v, want full catalog", got)
	}
	if got := resolve(community); !reflect.DeepEqual(got, []string{"process.spawn", "session.read"}) {
		t.Fatalf("community grant = %v", got)
	}
	mismatched := community
	mismatched.Fingerprint = "fp-2"
	if got := resolve(mismatched); got != nil {
		t.Fatalf("fingerprint-changed grant = %v, want nil", got)
	}
}
