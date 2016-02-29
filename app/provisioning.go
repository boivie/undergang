package app
import "time"

func isProvisioned(info *PathInfo) bool {
	return info.Provisioning == nil || info.Provisioning.Status != "started"
}

func waitProvisioning(origInfo *PathInfo, done chan *PathInfo, progress chan <- ProgressCmd) {
	progress <- ProgressCmd{"wait_provisioning_start", nil}
	for {
		newInfo := doLookup(origInfo.Host, origInfo.Prefix)
		if newInfo == nil || isProvisioned(newInfo) {
			done <- newInfo
			progress <- ProgressCmd{"wait_provisioning_end", nil}
			return
		}

		time.Sleep(5 * time.Second)
	}
}