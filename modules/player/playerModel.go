package player

import "time"

type (
	PlayerProfile struct {
		Id       string    `json:"id" bson:"_id"`
		Email    string    `json:"email"`
		Username string    `json:"username"`
		CreateAt time.Time `json:"create_at"`
		UpdateAt time.Time `json:"update_at"`
	}

	PlayerClaims struct {
		Id       string `json:"id"`
		RoleCode string `json:"role_code"`
	}

	CreatePlayerReq struct {
		Email    string `json:"email" from:"email" validate:"required,email,max=255"`
		Password string `json:"password" from:"password" validate:"required,max=32"`
		Username string `json:"username" from:"username" validate:"required,max=32"`
	}

	CreatePlayerTransactionReq struct {
		PlayerId string `json:"player_id" validate:"required,max=64"`
		Amount   string `json:"amount" validate:"required"`
	}
)
