package password

type Hasher interface {
	Hash(password string) (string, error)

	Compare(
		password string,
		encodedHash string,
	) (bool, error)
}
