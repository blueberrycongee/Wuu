package pluginhost

import "github.com/blueberrycongee/wuu/internal/extensions"

// protocolHookRequiredPermissions maps every interception-capable protocol
// hook to the closed-catalog permissions it requires. The mapping is owned by
// the host: manifest text or plugin declarations can never shrink it. Hooks
// absent from the map (session.start, session.stop) carry lifecycle metadata
// only and are always free.
var protocolHookRequiredPermissions = map[Hook][]string{
	HookChatMessage:       {extensions.PermSessionRead},
	HookChatRequest:       {extensions.PermSessionRead, extensions.PermSessionWrite},
	HookToolDefinition:    {extensions.PermToolsDefine},
	HookToolExecuteBefore: {extensions.PermToolsIntercept},
	HookToolExecuteAfter:  {extensions.PermToolsIntercept},
	HookShellEnv:          {extensions.PermShellEnv},
}

// RequiredPermissionsForProtocolHook returns the permissions a plugin must be
// granted to register hook. A nil result means the hook is metadata-only and
// requires no permission.
func RequiredPermissionsForProtocolHook(hook Hook) []string {
	required := protocolHookRequiredPermissions[hook]
	return append([]string(nil), required...)
}

// FilterHooksByGrantedPermissions splits declared hooks into the subset the
// granted permission set allows and the subset that must be stripped. Grant
// semantics are fail closed: a hook is kept only when every required
// permission is present in granted.
func FilterHooksByGrantedPermissions(declared []Hook, granted []string) (kept, stripped []Hook) {
	grantedSet := make(map[string]struct{}, len(granted))
	for _, permission := range granted {
		grantedSet[permission] = struct{}{}
	}
	for _, hook := range declared {
		required := protocolHookRequiredPermissions[hook]
		allowed := true
		for _, permission := range required {
			if _, ok := grantedSet[permission]; !ok {
				allowed = false
				break
			}
		}
		if allowed {
			kept = append(kept, hook)
		} else {
			stripped = append(stripped, hook)
		}
	}
	return kept, stripped
}
