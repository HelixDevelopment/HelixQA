// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package uinput

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctl numbers for /dev/uinput (see package doc).
const (
	iocUIDevCreate  = 0x5501
	iocUIDevDestroy = 0x5502
	iocUIDevSetup   = 0x405c5503
	iocUISetEvBit   = 0x40045564
	iocUISetKeyBit  = 0x40045565
	iocUISetRelBit  = 0x40045566
	iocUISetAbsBit  = 0x40045567
)

const (
	busVirtual     = 0x06
	uinputNameMax  = 80
	uinputSetupLen = 92 // sizeof(struct uinput_setup)
)

// Config describes a virtual input device.
type Config struct {
	Name    string // up to 80 bytes; truncated if longer
	Vendor  uint16 // defaults to 0x1234 when zero
	Product uint16 // defaults to 0x5678 when zero
	Version uint16 // defaults to 1 when zero
	BusType uint16 // defaults to BUS_VIRTUAL (0x06) when zero

	// EnableKey / EnableRel / EnableAbs gate the EV_ bits we advertise. Any of
	// the *Bits slices require the corresponding Enable* flag; inconsistent
	// configuration returns an error at Open time.
	EnableKey bool
	EnableRel bool
	EnableAbs bool

	// KeyBits, RelBits, AbsBits — the specific codes the device advertises.
	// Duplicate codes are OK but inefficient.
	KeyBits []uint16
	RelBits []uint16
	AbsBits []uint16

	// Only meaningful when EnableAbs is true: range bounds common to all
	// declared absolute axes. Finer-grained per-axis ranges (via
	// UI_ABS_SETUP) can be added later if needed.
	AbsMin int32
	AbsMax int32
}

// Device is a created virtual input device. Call Close to destroy it.
type Device struct {
	f       *os.File
	name    string
	created bool
}

// Open creates a virtual device from cfg and returns a Device that is ready
// to accept events via the WriteX helpers in this package.
//
// The caller MUST have read+write access to /dev/uinput — typically via a
// udev rule that grants group access (no "sudo" required at runtime).
func Open(cfg Config) (*Device, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, errors.New("uinput: empty name")
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("uinput: open /dev/uinput: %w", err)
	}
	d := &Device{f: f, name: cfg.Name}
	if err := d.configure(cfg); err != nil {
		_ = d.f.Close()
		return nil, err
	}
	d.created = true
	return d, nil
}

// Writer returns the underlying writer so callers can use WriteEvent /
// WriteKeyTap / WriteClickAbs / WriteMoveRel directly. After Close the
// returned Writer is invalid.
func (d *Device) Writer() io.Writer { return d.f }

// Close destroys the virtual device and closes /dev/uinput.
// Safe to call multiple times.
func (d *Device) Close() error {
	if d == nil || d.f == nil {
		return nil
	}
	var firstErr error
	if d.created {
		if _, _, e := unix.Syscall(unix.SYS_IOCTL, d.f.Fd(), iocUIDevDestroy, 0); e != 0 {
			firstErr = fmt.Errorf("uinput: UI_DEV_DESTROY: %w", e)
		}
		d.created = false
	}
	if err := d.f.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("uinput: close /dev/uinput: %w", err)
	}
	d.f = nil
	return firstErr
}

// Name returns the advertised device name.
func (d *Device) Name() string {
	if d == nil {
		return ""
	}
	return d.name
}

func validateConfig(cfg Config) error {
	if !cfg.EnableKey && len(cfg.KeyBits) > 0 {
		return errors.New("uinput: KeyBits set but EnableKey=false")
	}
	if !cfg.EnableRel && len(cfg.RelBits) > 0 {
		return errors.New("uinput: RelBits set but EnableRel=false")
	}
	if !cfg.EnableAbs && len(cfg.AbsBits) > 0 {
		return errors.New("uinput: AbsBits set but EnableAbs=false")
	}
	if cfg.EnableAbs && cfg.AbsMin >= cfg.AbsMax {
		return fmt.Errorf("uinput: bad abs range [%d,%d]", cfg.AbsMin, cfg.AbsMax)
	}
	return nil
}

func (d *Device) configure(cfg Config) error {
	fd := d.f.Fd()
	enable := func(evType uint32) error {
		if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, iocUISetEvBit, uintptr(evType)); e != 0 {
			return fmt.Errorf("uinput: UI_SET_EVBIT(%d): %w", evType, e)
		}
		return nil
	}
	setCap := func(ioc uintptr, codes []uint16) error {
		for _, c := range codes {
			if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, ioc, uintptr(c)); e != 0 {
				return fmt.Errorf("uinput: UI_SET_*BIT(%d): %w", c, e)
			}
		}
		return nil
	}
	// EV_SYN must always be enabled.
	if err := enable(uint32(EventTypeSyn)); err != nil {
		return err
	}
	if cfg.EnableKey {
		if err := enable(uint32(EventTypeKey)); err != nil {
			return err
		}
		if err := setCap(iocUISetKeyBit, cfg.KeyBits); err != nil {
			return err
		}
	}
	if cfg.EnableRel {
		if err := enable(uint32(EventTypeRel)); err != nil {
			return err
		}
		if err := setCap(iocUISetRelBit, cfg.RelBits); err != nil {
			return err
		}
	}
	if cfg.EnableAbs {
		if err := enable(uint32(EventTypeAbs)); err != nil {
			return err
		}
		if err := setCap(iocUISetAbsBit, cfg.AbsBits); err != nil {
			return err
		}
	}
	// UI_DEV_SETUP
	var setup [uinputSetupLen]byte
	vendor := cfg.Vendor
	if vendor == 0 {
		vendor = 0x1234
	}
	product := cfg.Product
	if product == 0 {
		product = 0x5678
	}
	version := cfg.Version
	if version == 0 {
		version = 1
	}
	bus := cfg.BusType
	if bus == 0 {
		bus = busVirtual
	}
	binary.LittleEndian.PutUint16(setup[0:2], bus)
	binary.LittleEndian.PutUint16(setup[2:4], vendor)
	binary.LittleEndian.PutUint16(setup[4:6], product)
	binary.LittleEndian.PutUint16(setup[6:8], version)
	name := cfg.Name
	if len(name) > uinputNameMax-1 {
		name = name[:uinputNameMax-1] // keep room for NUL
	}
	copy(setup[8:8+uinputNameMax], name)
	// ff_effects_max at 8+80 = 88 is already zeroed.
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, iocUIDevSetup, uintptr(unsafe.Pointer(&setup[0]))); e != 0 {
		return fmt.Errorf("uinput: UI_DEV_SETUP: %w", e)
	}
	// UI_DEV_CREATE
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, iocUIDevCreate, 0); e != 0 {
		return fmt.Errorf("uinput: UI_DEV_CREATE: %w", e)
	}
	return nil
}
