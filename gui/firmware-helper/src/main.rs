use espflash::{
    command::Command,
    connection::{Connection, ResetAfterOperation, ResetBeforeOperation},
    flasher::{FlashSize, Flasher},
    target::{Chip, ProgressCallbacks},
};
use firmware_helper::*;
use serialport::{SerialPortInfo, SerialPortType};
use std::io::{self, Write};
use std::time::Duration;

fn candidates() -> Result<Vec<SerialPortInfo>> {
    let mut ports = serialport::available_ports()
        .map_err(|_| Failure("SCAN_FAILED", "Cannot enumerate USB ports"))?;
    ports.retain(|p| matches!(&p.port_type, SerialPortType::UsbPort(u) if allowed_port(&p.port_name, u.vid, u.pid)));
    ports.sort_by(|a, b| a.port_name.cmp(&b.port_name));
    ports.dedup_by(|a, b| a.port_name == b.port_name);
    Ok(ports)
}
fn progress(out: &mut impl Write, stage: Stage, percent: u8, message: &'static str) -> Result<()> {
    emit(
        out,
        &Event::Progress {
            stage,
            percent: percent.min(100),
            message,
        },
    )
    .map_err(|_| Failure("OUTPUT_FAILED", "Cannot deliver progress"))
}
struct Callbacks<'a, W: Write> {
    out: &'a mut W,
    total: usize,
    failed: bool,
    verified: bool,
    verification_started: bool,
}
impl<W: Write> Callbacks<'_, W> {
    fn send(&mut self, stage: Stage, percent: u8, message: &'static str) {
        self.failed |= progress(self.out, stage, percent, message).is_err();
    }
}
impl<W: Write> ProgressCallbacks for Callbacks<'_, W> {
    fn init(&mut self, _addr: u32, total: usize) {
        self.total = total;
        self.send(Stage::Writing, 0, "Writing factory application");
    }
    fn update(&mut self, current: usize) {
        let percent = current
            .saturating_mul(100)
            .checked_div(self.total)
            .unwrap_or(0)
            .min(100) as u8;
        self.send(Stage::Writing, percent, "Writing factory application");
    }
    fn verifying(&mut self) {
        self.verification_started = true;
        self.send(Stage::Verifying, 0, "Verifying device flash checksum");
    }
    fn finish(&mut self, skipped: bool) {
        self.verified = self.verification_started && !skipped;
        if self.verified {
            self.send(Stage::Verifying, 100, "Device flash checksum verified");
            self.send(Stage::Restarting, 0, "Restarting device");
        }
    }
}

// Only the partition table is readable. Cardid contents are never requested.
fn read_preflight(flasher: &mut Flasher, offset: u32, size: u32) -> Result<Vec<u8>> {
    if (offset, size) != (0x8000, 0x1000) {
        return Err(Failure(
            "UNSAFE_READ",
            "Read outside approved preflight regions",
        ));
    }
    let mut bytes = Vec::with_capacity(size as usize);
    for at in (offset..offset + size).step_by(ROM_READ_BLOCK) {
        let response = flasher
            .connection()
            .command(Command::ReadFlashSlow {
                offset: at,
                size: ROM_READ_BLOCK as u32,
                block_size: ROM_READ_BLOCK as u32,
                max_in_flight: 1,
            })
            .map_err(|_| {
                Failure(
                    "PREFLIGHT_READ_UNAVAILABLE",
                    "ROM flash read unavailable; installation refused",
                )
            })?;
        let chunk: Vec<u8> = response
            .try_into()
            .map_err(|_| Failure("PREFLIGHT_READ_UNAVAILABLE", "Invalid ROM read response"))?;
        bytes.extend_from_slice(&rom_read_payload(chunk)?);
    }
    Ok(bytes)
}
fn install(
    port: &str,
    firmware_path: &str,
    manifest_path: &str,
    out: &mut impl Write,
) -> Result<()> {
    progress(out, Stage::Preflight, 0, "Validating firmware and manifest")?;
    let firmware = read_file(firmware_path, APP_MAX as usize)?;
    let manifest: Manifest =
        serde_json::from_slice(&read_file(manifest_path, 65536)?).map_err(|_| {
            Failure(
                "INVALID_MANIFEST",
                "Manifest is not valid schema version 1 JSON",
            )
        })?;
    validate_manifest(&manifest, &firmware)?;
    // Re-enumerate at install time; never trust a UI supplied identifier alone.
    let selected = candidates()?
        .into_iter()
        .find(|p| p.port_name == port)
        .ok_or(Failure(
            "PORT_NOT_ALLOWED",
            "Port is not a supported native USB Serial/JTAG device",
        ))?;
    let SerialPortType::UsbPort(mut usb) = selected.port_type else {
        unreachable!()
    };
    usb.serial_number = None;
    let serial = serialport::new(&selected.port_name, 115200)
        .timeout(Duration::from_secs(3))
        .open_native()
        .map_err(|_| Failure("CONNECT_FAILED", "Cannot open the selected USB device"))?;
    let connection = Connection::new(
        serial,
        usb,
        ResetAfterOperation::HardReset,
        ResetBeforeOperation::UsbReset,
        115200,
    );
    // ROM only: no stub upload before the security state has been checked.
    // verify=true and skip=false guarantee device checksum verification on every write.
    let mut flasher =
        Flasher::connect(connection, false, true, false, Some(Chip::Esp32c3), None)
            .map_err(|_| Failure("CONNECT_FAILED", "Cannot reliably connect to an ESP32-C3"))?;
    let preflight = (|| {
        if flasher.chip() != Chip::Esp32c3 {
            return Err(Failure("CHIP_MISMATCH", "Expected ESP32-C3"));
        }
        let info = flasher
            .security_info()
            .map_err(|_| Failure("SECURITY_UNAVAILABLE", "Cannot read device security state"))?;
        validate_security(
            info.flags,
            info.flash_crypt_cnt,
            info.chip_id,
            flasher.secure_download_mode(),
        )?;
        if flasher
            .flash_detect()
            .map_err(|_| Failure("FLASH_SIZE_UNAVAILABLE", "Cannot detect flash size"))?
            != Some(FlashSize::_8Mb)
        {
            return Err(Failure(
                "FLASH_SIZE_MISMATCH",
                "An explicitly detected 8 MiB flash is required",
            ));
        }
        let actual = parse_partition_table(&read_preflight(&mut flasher, 0x8000, 0x1000)?)?;
        // The manifest describes the packaged app, not a partition table to
        // install. USB ROM writes need only the validated factory slot; the
        // existing bootloader and Recovery are preserved without modification.
        let plan = plan_app_write(&actual, manifest.app_size)?;
        progress(out, Stage::Preflight, 100, "Device safety checks passed")?;
        Ok(plan)
    })();
    let plan = match preflight {
        Ok(plan) => plan,
        Err(failure) => {
            // A refused preflight has not written flash. Leave the existing
            // application bootable instead of stranding the board in ROM mode.
            let _ = flasher.connection().reset_after(false, Chip::Esp32c3);
            return Err(failure);
        }
    };
    let mut callbacks = Callbacks {
        out,
        total: 0,
        failed: false,
        verified: false,
        verification_started: false,
    };
    // Sole flash mutation: one factory app segment. espflash pads at most three 0xff bytes.
    flasher
        .write_bin_to_flash(plan.offset, &firmware, &mut callbacks)
        .map_err(|_| {
            Failure(
                "FLASH_FAILED",
                "Application write, device verification, or reset failed",
            )
        })?;
    if !callbacks.verified {
        return Err(Failure(
            "VERIFICATION_UNAVAILABLE",
            "Device verification was not completed",
        ));
    }
    if callbacks.failed {
        return Err(Failure("OUTPUT_FAILED", "Cannot deliver progress"));
    }
    progress(
        callbacks.out,
        Stage::Restarting,
        100,
        "Device reset completed",
    )?;
    Ok(())
}
fn run() -> io::Result<()> {
    let stdin = io::stdin();
    let mut input = stdin.lock();
    let stdout = io::stdout();
    let mut out = stdout.lock();
    emit(&mut out, &Event::Ready)?;
    while let Some(line) = next_line(&mut input)? {
        let request = line.and_then(|bytes| {
            serde_json::from_slice::<Request>(&bytes)
                .map_err(|_| Failure("INVALID_REQUEST", "Invalid NDJSON request"))
        });
        let result = match request {
            Ok(Request::Shutdown {}) => break,
            Ok(Request::Scan {}) => candidates().and_then(|ports| {
                let devices = ports
                    .into_iter()
                    .map(|p| Device {
                        id: p.port_name,
                        name: "Go Voice USB Serial/JTAG",
                    })
                    .collect();
                emit(&mut out, &Event::Ports { devices })
                    .map_err(|_| Failure("OUTPUT_FAILED", "Cannot deliver device list"))
            }),
            Ok(Request::Install {
                port,
                firmware_path,
                manifest_path,
            }) => install(&port, &firmware_path, &manifest_path, &mut out).and_then(|()| {
                emit(&mut out, &Event::Complete)
                    .map_err(|_| Failure("OUTPUT_FAILED", "Cannot deliver completion"))
            }),
            Err(error) => Err(error),
        };
        if let Err(Failure(code, message)) = result {
            emit(
                &mut out,
                &Event::Error {
                    code,
                    message,
                    recoverable: code != "OUTPUT_FAILED",
                },
            )?;
        }
    }
    Ok(())
}
fn main() {
    if run().is_err() {
        std::process::exit(1);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn callbacks_report_ordered_verification_without_false_completion() {
        let mut out = Vec::new();
        let mut callbacks = Callbacks {
            out: &mut out,
            total: 0,
            failed: false,
            verified: false,
            verification_started: false,
        };
        callbacks.init(APP_OFFSET, 2);
        callbacks.update(1);
        callbacks.update(usize::MAX);
        callbacks.finish(false);
        assert!(!callbacks.verified);
        callbacks.verifying();
        callbacks.finish(true);
        assert!(!callbacks.verified);
        callbacks.finish(false);
        assert!(callbacks.verified);
        assert!(!callbacks.failed);
        let events: Vec<serde_json::Value> = String::from_utf8(out)
            .unwrap()
            .lines()
            .map(|s| serde_json::from_str(s).unwrap())
            .collect();
        assert_eq!(events.len(), 6);
        assert_eq!(events[0]["stage"], "writing");
        assert_eq!(events[1]["percent"], 50);
        assert_eq!(events[2]["percent"], 100);
        assert_eq!(events[3]["stage"], "verifying");
        assert_eq!(events[4]["percent"], 100);
        assert_eq!(events[5]["stage"], "restarting");
        assert!(events.iter().all(|e| e["type"] == "progress"));
    }
}
