package app
import "time"

func waitProvisioning(origInfo *PathInfo, done chan *PathInfo) {
	info := origInfo
	for {
		if info != nil && (info.Provisioning == nil || info.Provisioning.Status != "started") {
			done <- info
			return
		}

		time.Sleep(5 * time.Second)

		info = doLookup(origInfo.Prefix)
	}
}