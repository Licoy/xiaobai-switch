//! The 0.0.12 bridge keeps macOS 11 on Tauri and opts supported systems into native updates.
pub const ENDPOINT: &str =
    "https://github.com/Licoy/xiaobai-switch/releases/latest/download/native-transition.json";

pub fn eligible(version: &str, os: &str, major: u32) -> bool {
    version == "0.0.12"
        && match os {
            "macos" => major >= 12,
            "windows" => major >= 10,
            _ => false,
        }
}

#[cfg(target_os = "macos")]
pub fn os_major() -> Result<u32, String> {
    let output = std::process::Command::new("/usr/bin/sw_vers")
        .arg("-productVersion")
        .output()
        .map_err(|e| e.to_string())?;
    if !output.status.success() {
        return Err("cannot determine macOS version".into());
    }
    parse_major(std::str::from_utf8(&output.stdout).map_err(|e| e.to_string())?)
}
#[cfg(target_os = "windows")]
pub fn os_major() -> Result<u32, String> {
    #[repr(C)]
    struct Version {
        size: u32,
        major: u32,
        minor: u32,
        build: u32,
        platform: u32,
        service: [u16; 128],
    }
    #[link(name = "ntdll")]
    extern "system" {
        fn RtlGetVersion(info: *mut Version) -> i32;
    }
    let mut info = Version {
        size: std::mem::size_of::<Version>() as u32,
        major: 0,
        minor: 0,
        build: 0,
        platform: 0,
        service: [0; 128],
    };
    let status = unsafe { RtlGetVersion(&mut info) };
    if status != 0 {
        return Err(format!("cannot determine Windows version: {status}"));
    }
    Ok(info.major)
}
#[cfg(not(any(target_os = "macos", target_os = "windows")))]
pub fn os_major() -> Result<u32, String> {
    Ok(0)
}

#[cfg(any(target_os = "macos", test))]
fn parse_major(value: &str) -> Result<u32, String> {
    value
        .trim()
        .split('.')
        .next()
        .ok_or("missing OS version")?
        .parse()
        .map_err(|_| "invalid OS version".into())
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn bridge_support_gate() {
        assert!(!eligible("0.0.11", "macos", 15));
        assert!(!eligible("0.0.12", "macos", 11));
        assert!(eligible("0.0.12", "macos", 12));
        assert!(eligible("0.0.12", "windows", 10));
        assert!(!eligible("0.0.12", "windows", 6));
        assert!(!eligible("0.0.12", "linux", 12));
    }
    #[test]
    fn version_parser() {
        assert_eq!(parse_major("12.7.6\n").unwrap(), 12);
        assert!(parse_major("unknown").is_err());
    }
}
