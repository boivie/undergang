package app

var externalLookupUrl string
var proxyCommand string

func Init(externalPathLookupUrl_ string, accessLookupUrl string, proxyCommand_ string) {
	proxyCommand = proxyCommand_
	externalLookupUrl = externalPathLookupUrl_
	go accessManager(accessLookupUrl)
	go backendManager()
}
