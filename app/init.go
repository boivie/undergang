package app

func Init(externalPathLookupUrl string, accessLookupUrl string, proxyCommand string) {
	go accessManager(accessLookupUrl)
	go pathManager(externalPathLookupUrl)
	go sshMuxer(proxyCommand)
}
