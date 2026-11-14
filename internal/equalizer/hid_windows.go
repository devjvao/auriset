package equalizer

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	cmInterfacesPresent = 0
	cmBufferSmall       = 0x1a
	hidOutput           = 1
	hidSuccess          = 0x00110000
)

// HIDD_ATTRIBUTES and HIDP_CAPS use fixed-width fields on every Windows architecture.
type hidAttributes struct {
	size    uint32
	vendor  uint16
	product uint16
	version uint16
}

type hidCapabilities struct {
	usage               uint16
	usagePage           uint16
	inputReportLength   uint16
	outputReportLength  uint16
	featureReportLength uint16
	_                   [21]uint16 // Reserved[17], link count, and three input counts.
	outputButtonCaps    uint16
	outputValueCaps     uint16
	_                   [4]uint16
}

// HIDP_VALUE_CAPS and HIDP_BUTTON_CAPS are each 72 bytes. Both have
// ReportID at byte 2. The remaining fields are unused here.
type hidReportCapability struct {
	usagePage uint16
	reportID  byte
	_         byte
	_         [17]uint32
}

type windowsHID struct {
	getGUID              *syscall.LazyProc
	getInterfaceListSize *syscall.LazyProc
	getInterfaceList     *syscall.LazyProc
	getAttributes        *syscall.LazyProc
	getPreparsedData     *syscall.LazyProc
	freePreparsedData    *syscall.LazyProc
	getCaps              *syscall.LazyProc
	getValueCaps         *syscall.LazyProc
	getButtonCaps        *syscall.LazyProc
}

var loadWindowsHID = sync.OnceValues(func() (*windowsHID, error) {
	// Use absolute system DLL paths so the working directory is never searched.
	getSystemDirectory := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW")
	var directory [syscall.MAX_PATH]uint16
	n, _, err := getSystemDirectory.Call(uintptr(unsafe.Pointer(&directory[0])), uintptr(len(directory)))
	if n == 0 {
		return nil, fmt.Errorf("GetSystemDirectoryW: %w", err)
	}
	if n >= uintptr(len(directory)) {
		return nil, fmt.Errorf("system directory path is too long")
	}
	systemDirectory := syscall.UTF16ToString(directory[:n])
	hid := syscall.NewLazyDLL(filepath.Join(systemDirectory, "hid.dll"))
	cm := syscall.NewLazyDLL(filepath.Join(systemDirectory, "cfgmgr32.dll"))
	api := &windowsHID{
		getGUID:              hid.NewProc("HidD_GetHidGuid"),
		getInterfaceListSize: cm.NewProc("CM_Get_Device_Interface_List_SizeW"),
		getInterfaceList:     cm.NewProc("CM_Get_Device_Interface_ListW"),
		getAttributes:        hid.NewProc("HidD_GetAttributes"),
		getPreparsedData:     hid.NewProc("HidD_GetPreparsedData"),
		freePreparsedData:    hid.NewProc("HidD_FreePreparsedData"),
		getCaps:              hid.NewProc("HidP_GetCaps"),
		getValueCaps:         hid.NewProc("HidP_GetValueCaps"),
		getButtonCaps:        hid.NewProc("HidP_GetButtonCaps"),
	}
	for _, proc := range []*syscall.LazyProc{
		api.getGUID, api.getInterfaceListSize, api.getInterfaceList, api.getAttributes,
		api.getPreparsedData, api.freePreparsedData, api.getCaps, api.getValueCaps, api.getButtonCaps,
	} {
		if err := proc.Find(); err != nil {
			return nil, err
		}
	}
	return api, nil
})

type hidDevice struct {
	path               string
	outputReportLength int
}

// NewApp creates the equalizer service with Windows HID cable discovery.
func NewApp(historyPath string) *App {
	return newApp(historyPath, findAuri)
}

func findAuri() ([]openCable, error) {
	api, err := loadWindowsHID()
	if err != nil {
		return nil, err
	}
	paths, err := api.interfacePaths()
	if err != nil {
		return nil, err
	}
	var devices []openCable
	for _, path := range paths {
		length, err := api.auriReportLength(path)
		if err != nil {
			return nil, err
		}
		if length != 0 {
			device := hidDevice{path: path, outputReportLength: length}
			devices = append(devices, device.open)
		}
	}
	return devices, nil
}

func (api *windowsHID) interfacePaths() ([]string, error) {
	var guid syscall.GUID
	_, _, _ = api.getGUID.Call(uintptr(unsafe.Pointer(&guid))) // HidD_GetHidGuid returns void.
	for range 3 {
		var size uint32
		status, _, _ := api.getInterfaceListSize.Call(
			uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&guid)), 0, cmInterfacesPresent,
		)
		if uint32(status) != 0 {
			return nil, fmt.Errorf("get HID interface list size: CONFIGRET %#x", uint32(status))
		}
		if size <= 1 {
			return nil, nil
		}
		list := make([]uint16, size)
		status, _, _ = api.getInterfaceList.Call(
			uintptr(unsafe.Pointer(&guid)), 0, uintptr(unsafe.Pointer(&list[0])), uintptr(size), cmInterfacesPresent,
		)
		switch uint32(status) {
		case 0:
			// The interface list is a sequence of NUL-terminated UTF-16 paths.
			var paths []string
			start := 0
			for end, unit := range list {
				if unit == 0 {
					if end == start {
						break
					}
					paths = append(paths, syscall.UTF16ToString(list[start:end]))
					start = end + 1
				}
			}
			return paths, nil
		case cmBufferSmall:
			// An interface arrived between the two calls. Get a fresh size.
			continue
		default:
			return nil, fmt.Errorf("get HID interface list: CONFIGRET %#x", uint32(status))
		}
	}
	return nil, fmt.Errorf("HID interface list changed repeatedly during detection")
}

func (api *windowsHID) auriReportLength(path string) (int, error) {
	// Metadata access alone permits enumeration without claiming a write handle.
	handle, err := openHIDPath(path, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE)
	if err != nil {
		// The path is only a hint for diagnostics when attributes are unavailable.
		id := fmt.Sprintf("vid_%04x&pid_%04x", vendorID, productID)
		if strings.Contains(strings.ToLower(path), id) {
			return 0, fmt.Errorf("open Auri HID metadata: %w", err)
		}
		return 0, nil // A collection may be inaccessible or have disappeared.
	}
	defer func() { _ = syscall.CloseHandle(handle) }()

	var attrs hidAttributes
	attrs.size = uint32(unsafe.Sizeof(attrs))
	ok, _, _ := api.getAttributes.Call(uintptr(handle), uintptr(unsafe.Pointer(&attrs)))
	if byte(ok) == 0 || attrs.vendor != vendorID || attrs.product != productID {
		return 0, nil
	}

	var preparsed uintptr
	ok, _, err = api.getPreparsedData.Call(uintptr(handle), uintptr(unsafe.Pointer(&preparsed)))
	if byte(ok) == 0 {
		return 0, fmt.Errorf("get Auri HID preparsed data: %w", err)
	}
	defer func() { _, _, _ = api.freePreparsedData.Call(preparsed) }()
	var caps hidCapabilities
	status, _, _ := api.getCaps.Call(preparsed, uintptr(unsafe.Pointer(&caps)))
	if uint32(status) != hidSuccess {
		return 0, fmt.Errorf("get Auri HID capabilities: NTSTATUS %#x", uint32(status))
	}
	if int(caps.outputReportLength) < nativeReportLen {
		return 0, nil
	}

	found, err := hasAuriOutputReport(api.getValueCaps, preparsed, caps.outputValueCaps)
	if err != nil {
		return 0, err
	}
	if !found {
		found, err = hasAuriOutputReport(api.getButtonCaps, preparsed, caps.outputButtonCaps)
		if err != nil {
			return 0, err
		}
	}
	if !found {
		return 0, nil
	}
	return int(caps.outputReportLength), nil
}

func hasAuriOutputReport(getCaps *syscall.LazyProc, preparsed uintptr, count uint16) (bool, error) {
	if count == 0 {
		return false, nil
	}
	outputCaps := make([]hidReportCapability, count)
	status, _, _ := getCaps.Call(
		hidOutput, uintptr(unsafe.Pointer(&outputCaps[0])), uintptr(unsafe.Pointer(&count)), preparsed,
	)
	if uint32(status) != hidSuccess {
		return false, fmt.Errorf("%s: NTSTATUS %#x", getCaps.Name, uint32(status))
	}
	if int(count) > len(outputCaps) {
		return false, fmt.Errorf("%s returned an invalid capability count", getCaps.Name)
	}
	for _, cap := range outputCaps[:count] {
		if cap.reportID == reportID {
			return true, nil
		}
	}
	return false, nil
}

func openHIDPath(path string, access, share uint32) (syscall.Handle, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	return syscall.CreateFile(name, access, share, nil, syscall.OPEN_EXISTING, 0, 0)
}

type hidWriter struct {
	handle syscall.Handle
	buffer []byte
}

func (d hidDevice) open() (io.WriteCloser, error) {
	// Windows checks sharing against handles that are already open. Use the
	// normal HID sharing mode so an existing host handle does not block access.
	handle, err := openHIDPath(d.path, syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE)
	if err != nil {
		return nil, fmt.Errorf("open Auri HID for writing: %w", err)
	}
	return &hidWriter{handle: handle, buffer: make([]byte, d.outputReportLength)}, nil
}

func (w *hidWriter) Write(report []byte) (int, error) {
	if len(report) == 0 || len(report) > len(w.buffer) || report[0] != reportID {
		return 0, fmt.Errorf("invalid Auri HID output report")
	}
	// Windows requires the full collection output report length, even for apply.
	clear(w.buffer)
	copy(w.buffer, report)
	var written uint32
	err := syscall.WriteFile(w.handle, w.buffer, &written, nil)
	if err != nil {
		return 0, err
	}
	if int(written) != len(w.buffer) {
		return min(int(written), len(report)), io.ErrShortWrite
	}
	return len(report), nil
}

func (w *hidWriter) Close() error {
	return syscall.CloseHandle(w.handle)
}
