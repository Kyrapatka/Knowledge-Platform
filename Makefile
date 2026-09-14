DB_URL=postgres://postgres:hell@localhost:5432/knowledge_platform?sslmode=disable

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1

