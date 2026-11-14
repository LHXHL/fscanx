package common

import (
	"fmt"
	"os"
	"strings"
)

// usesStdinLikePipeline 表示当前运行会走标准输入管线（-std 或 -msc）。
// 只用于派生初始化，不能反过来设置 ScanWithStdInput，否则 main 会去读 os.Stdin。
func usesStdinLikePipeline() bool {
	return ScanWithStdInput || MasscanMode
}

// ValidateMasscanMode 校验 -msc 与其它参数的组合。必须在 common.Parse 之前调用，
// 避免 ParseInput 在缺少目标时先以 0 退出。
func ValidateMasscanMode(host string) {
	if !MasscanMode {
		return
	}

	var reason string
	switch {
	case ScanWithStdInput:
		reason = "-msc and -std cannot be used together"
	case DomainPortBind:
		reason = "-msc cannot be used with -dp (masscan targets only come from -h/-hf)"
	case URL != "" || UrlFile != "":
		reason = "-msc cannot be used with -u/-uf (masscan targets only come from -h/-hf)"
	case Scantype == "icmp": // 须先于 AutoScanBigCidr：switch 无 fallthrough，组合下也要拒绝
		reason = "-msc cannot be used with -m icmp (no ports to scan)"
	case MasscanRate <= 0: // 同上，组合下 -mr<=0 也要拒绝
		reason = "-mr must be greater than 0"
	case AutoScanBigCidr:
		// -msc -auto 组合（BL-MSC-004）：探活目标只来自 -h（AutoScanBigCidr 只读 Host），
		// -hf 会被静默忽略——显式拒绝优于静默。
		if host == "" {
			reason = "-msc -auto requires -h (big CIDR like 10.0.0.0/8)"
		} else if HostFile != "" {
			reason = "-msc -auto cannot be used with -hf (probe targets come from -h only)"
		}
	case host == "" && HostFile == "":
		reason = "-msc requires -h or -hf"
	}
	if reason != "" {
		fmt.Fprintln(os.Stderr, "[-]", reason)
		os.Exit(2)
	}

	if err := validateMasscanExtraArgs(MasscanArgs); err != nil {
		fmt.Fprintln(os.Stderr, "[-]", err)
		os.Exit(2)
	}
}

// validateMasscanExtraArgs 禁止 -margs 覆盖 fscanx 托管的参数，避免绕过端口相减、
// 目标转换和速率设置。
func validateMasscanExtraArgs(raw string) error {
	for _, arg := range strings.Fields(raw) {
		if IsMasscanManagedArg(arg) {
			return fmt.Errorf("masscan option %q is managed by fscanx; use -mr/-p/-h/-hf/-hn instead", arg)
		}
	}
	return nil
}

// IsMasscanManagedArg 判断一个 masscan 参数是否由 fscanx 托管（不允许通过 -margs 覆盖）。
func IsMasscanManagedArg(arg string) bool {
	if arg == "-p" || (strings.HasPrefix(arg, "-p") && !strings.HasPrefix(arg, "--")) {
		return true
	}
	if strings.HasPrefix(arg, "-iL") {
		return true
	}
	for _, name := range []string{"--ports", "--rate", "--includefile", "--exclude", "--excludefile"} {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}
