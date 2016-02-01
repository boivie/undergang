package app

func (b* backendStruct) waitProvisioning(done chan <- bool) {
	done <- true
}