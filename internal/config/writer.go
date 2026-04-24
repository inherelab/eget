package config

func Save(path string, file *File) error {
	return saveConfigFile(path, file)
}
