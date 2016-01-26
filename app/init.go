package app

func Init(externalLookupUrl string) {
	go pathManager(externalLookupUrl)
	go sshConnector()
}
