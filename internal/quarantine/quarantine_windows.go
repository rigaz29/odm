//go:build windows

package quarantine

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procCoCreateInstance = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")

var (
	clsidAttachmentServices = windows.GUID{Data1: 0x4125dd96, Data2: 0xe03a, Data3: 0x4103,
		Data4: [8]byte{0x8f, 0x70, 0xe0, 0x59, 0x7d, 0x80, 0x3b, 0x9c}}
	iidIAttachmentExecute = windows.GUID{Data1: 0x73db1241, Data2: 0x1e85, Data3: 0x4581,
		Data4: [8]byte{0x8e, 0x4f, 0xa8, 0x1e, 0x1d, 0x0f, 0x8c, 0x57}}
	// clientGUID names ODM to Attachment Services, which keeps the choices
	// a user makes in its prompts per program.
	clientGUID = windows.GUID{Data1: 0x6f1e3a52, Data2: 0x0d4b, Data3: 0x4c7e,
		Data4: [8]byte{0x9a, 0x31, 0x5e, 0x2c, 0x8b, 0x77, 0x40, 0x1d}}
)

// The IAttachmentExecute methods ODM calls, by their place in its vtable
// (after IUnknown's QueryInterface, AddRef and Release).
const (
	methodRelease       = 2
	methodSetClientGuid = 4
	methodSetLocalPath  = 5
	methodSetSource     = 7
	methodSetReferrer   = 8
	methodSave          = 11
)

const (
	clsctxInprocServer = 0x1
	rpcEChangedMode    = 0x80010106
)

// ErrRemoved means the file is gone after it was handed to Attachment
// Services: the antivirus program or a policy removed it.
var ErrRemoved = errors.New("Windows security removed the file")

type comObject struct{ vtbl *[16]uintptr }

func (o *comObject) call(method int, args ...uintptr) error {
	r, _, _ := syscall.SyscallN(o.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	if int32(r) < 0 {
		return fmt.Errorf("HRESULT 0x%08X", uint32(r))
	}
	return nil
}

// Mark records where the file at path came from. It hands the file to
// Windows Attachment Services, as browsers do: that writes the
// Zone.Identifier stream the system policy asks for and lets the
// registered antivirus program scan the file. Should the service be
// unavailable, the stream is written directly. ErrRemoved reports a file
// the scan removed.
func Mark(path, source, referrer string) error {
	src := sourceURL(source)
	ref := ""
	if strings.TrimSpace(referrer) != "" {
		ref = sourceURL(referrer)
	}
	err := attachmentServices(path, src, ref)
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		return ErrRemoved
	}
	if err == nil {
		return nil
	}
	if local(src) {
		return nil // no mark is due, and the service could not say otherwise
	}
	if werr := writeZoneIdentifier(path, src, ref); werr != nil {
		return fmt.Errorf("attachment services: %v; zone identifier: %w", err, werr)
	}
	return nil
}

// attachmentServices runs IAttachmentExecute.Save on path.
func attachmentServices(path, source, referrer string) error {
	// COM is set up per thread: keep this goroutine on one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	switch err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); {
	case err == nil, errors.Is(err, syscall.Errno(1)): // S_OK, S_FALSE (already set up)
		defer windows.CoUninitialize()
	case errors.Is(err, syscall.Errno(rpcEChangedMode)):
		// Set up in another mode on this thread; that one serves as well.
	default:
		return fmt.Errorf("CoInitializeEx: %w", err)
	}

	var ae *comObject
	r, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidAttachmentServices)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIAttachmentExecute)), uintptr(unsafe.Pointer(&ae)))
	if int32(r) < 0 || ae == nil {
		return fmt.Errorf("CoCreateInstance: HRESULT 0x%08X", uint32(r))
	}
	defer func() { _ = ae.call(methodRelease) }()

	if err := ae.call(methodSetClientGuid, uintptr(unsafe.Pointer(&clientGUID))); err != nil {
		return fmt.Errorf("SetClientGuid: %w", err)
	}
	if err := callString(ae, methodSetLocalPath, path); err != nil {
		return fmt.Errorf("SetLocalPath: %w", err)
	}
	if err := callString(ae, methodSetSource, source); err != nil {
		return fmt.Errorf("SetSource: %w", err)
	}
	if referrer != "" {
		if err := callString(ae, methodSetReferrer, referrer); err != nil {
			return fmt.Errorf("SetReferrer: %w", err)
		}
	}
	if err := ae.call(methodSave); err != nil {
		return fmt.Errorf("Save: %w", err)
	}
	return nil
}

func callString(o *comObject, method int, s string) error {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return err
	}
	err = o.call(method, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	return err
}

// writeZoneIdentifier writes the stream Attachment Services would: the
// file came from the internet zone, from source, linked from referrer.
func writeZoneIdentifier(path, source, referrer string) error {
	var b strings.Builder
	b.WriteString("[ZoneTransfer]\r\nZoneId=3\r\n")
	if referrer != "" {
		b.WriteString("ReferrerUrl=" + referrer + "\r\n")
	}
	if source != "" && source != "about:internet" {
		b.WriteString("HostUrl=" + source + "\r\n")
	}
	return os.WriteFile(path+":Zone.Identifier", []byte(b.String()), 0o644)
}
