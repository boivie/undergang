package app

func Init() {
	go pathManager()
	go sshConnector()
}
