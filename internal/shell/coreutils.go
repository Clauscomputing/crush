package shell

import (
	"os"
	"runtime"
	"strconv"
)

var useGoCoreUtils bool

func init() {
	// If CRUSH_CORE_UTILS is set to either true or false, respect that.
	// By default, enable on Windows and OpenBSD.
	// On OpenBSD, unveil restrictions are not properly inherited through exec,
	// so we use in-process coreutils to enforce filesystem sandboxing.
	if v, err := strconv.ParseBool(os.Getenv("CRUSH_CORE_UTILS")); err == nil {
		useGoCoreUtils = v
	} else {
		useGoCoreUtils = runtime.GOOS == "windows" || runtime.GOOS == "openbsd"
	}
}
