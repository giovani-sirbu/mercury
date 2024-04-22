package aggragates

type TradesExchanges struct {
	ID        uint   `gorm:"primaryKey" form:"id" json:"id" xml:"id"`
	Label     string `gorm:"type:varchar(50)" bson:"label" json:"label" form:"label" xml:"label" validate:"required,min=3,max=50"`
	Name      string `gorm:"type:varchar(50)" bson:"name" json:"name" form:"name" xml:"name" validate:"required,min=3,max=50"`
	ApiKey    string `gorm:"type:varchar(200)" bson:"apiKey" json:"apiKey" form:"apiKey" xml:"apiKey" validate:"omitempty,min=10,max=150"`
	ApiSecret string `gorm:"type:varchar(200)" bson:"apiSecret" json:"apiSecret" form:"apiSecret" xml:"apiSecret" validate:"omitempty,min=10,max=150"`
	TestNet   bool   `gorm:"type:boolean;default:false" bson:"testNet" json:"testNet" form:"testNet" xml:"testNet"`
}
