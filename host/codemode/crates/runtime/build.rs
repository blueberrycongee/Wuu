// Emits the Apple framework links the static V8 archive needs. The crates.io
// v8-150.4.0 build script stops at librusty_v8 + the C++ stdlib; V8's macOS
// partition_alloc and sandbox code reference CoreFoundation and friends, so
// the final link fails with "symbol(s) not found" (kCFBooleanTrue etc.)
// without these. The set matches the frameworks present on the verified codex
// host binary.
fn main() {
    if std::env::var("CARGO_CFG_TARGET_OS").as_deref() == Ok("macos") {
        for framework in [
            "CoreFoundation",
            "Foundation",
            "CFNetwork",
            "Security",
            "SystemConfiguration",
        ] {
            println!("cargo:rustc-link-lib=framework={framework}");
        }
        println!("cargo:rustc-link-lib=dylib=objc");
    }
    println!("cargo:rerun-if-changed=build.rs");
}
