package app

func Init(externalPathLookupUrl string, accessLookupUrl string, proxyCommand_ string) {
	proxyCommand = proxyCommand_
	go accessManager(accessLookupUrl)
	go backendManager(externalPathLookupUrl)
}
