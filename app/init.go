package app

func Init(externalPathLookupUrl string, accessLookupUrl string) {
	go accessManager(accessLookupUrl)
	go pathManager(externalPathLookupUrl)
	go sshConnector()
}
