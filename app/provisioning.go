package app
import "time"

func waitProvisioning(origInfo *PathInfo, done chan *PathInfo, progress chan <- ProgressCmd) {
	if origInfo.Provisioning == nil || origInfo.Provisioning.Status != "started" {
		done <- origInfo
		return
	}

	progress <- ProgressCmd{"wait_provisioning_start", nil}
	for {
		newInfo := doLookup(origInfo.Host, origInfo.Prefix)
		if newInfo != nil && (newInfo.Provisioning == nil || newInfo.Provisioning.Status != "started") {
			done <- newInfo
			progress <- ProgressCmd{"wait_provisioning_end", nil}
			return
		}

		time.Sleep(5 * time.Second)
	}
}