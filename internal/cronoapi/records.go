package cronoapi

import "time"

// ServingRecord is one row from the Cronometer "servings" CSV export —
// a single food item logged on a particular date.
//
// The named fields (RecordedTime, Group, FoodName, QuantityValue,
// QuantityUnits, Category) are the ones crono-export-cli's renderer
// consumes by name. Every other field on this struct is a nutrient
// column whose Go name follows the documented "<Name><Unit>" convention
// so that cmd/format.go's reflection walker can render them.
//
// The nutrient set below covers the columns Cronometer's public web
// export emits today. Columns Cronometer adds later that don't match
// a declared field name are dropped on parse — recapture WIRE_SHAPES.md
// and extend this struct when that happens.
type ServingRecord struct {
	RecordedTime  time.Time `json:"RecordedTime"`
	Group         string    `json:"Group"`
	FoodName      string    `json:"FoodName"`
	QuantityValue float64   `json:"QuantityValue"`
	QuantityUnits string    `json:"QuantityUnits"`
	Category      string    `json:"Category"`

	EnergyKcal       float64 `json:"EnergyKcal"`
	AlcoholG         float64 `json:"AlcoholG"`
	CaffeineMg       float64 `json:"CaffeineMg"`
	WaterG           float64 `json:"WaterG"`
	CarbsG           float64 `json:"CarbsG"`
	FiberG           float64 `json:"FiberG"`
	StarchG          float64 `json:"StarchG"`
	SugarsG          float64 `json:"SugarsG"`
	AddedSugarsG     float64 `json:"AddedSugarsG"`
	NetCarbsG        float64 `json:"NetCarbsG"`
	FatG             float64 `json:"FatG"`
	MonounsaturatedG float64 `json:"MonounsaturatedG"`
	PolyunsaturatedG float64 `json:"PolyunsaturatedG"`
	SaturatedG       float64 `json:"SaturatedG"`
	TransFatsG       float64 `json:"TransFatsG"`
	CholesterolMg    float64 `json:"CholesterolMg"`
	Omega3G          float64 `json:"Omega3G"`
	Omega6G          float64 `json:"Omega6G"`
	ProteinG         float64 `json:"ProteinG"`
	CystineG         float64 `json:"CystineG"`
	HistidineG       float64 `json:"HistidineG"`
	IsoleucineG      float64 `json:"IsoleucineG"`
	LeucineG         float64 `json:"LeucineG"`
	LysineG          float64 `json:"LysineG"`
	MethionineG      float64 `json:"MethionineG"`
	PhenylalanineG   float64 `json:"PhenylalanineG"`
	ThreonineG       float64 `json:"ThreonineG"`
	TryptophanG      float64 `json:"TryptophanG"`
	TyrosineG        float64 `json:"TyrosineG"`
	ValineG          float64 `json:"ValineG"`
	B1Mg             float64 `json:"B1Mg"`
	B2Mg             float64 `json:"B2Mg"`
	B3Mg             float64 `json:"B3Mg"`
	B5Mg             float64 `json:"B5Mg"`
	B6Mg             float64 `json:"B6Mg"`
	B12Ug            float64 `json:"B12Ug"`
	FolateUg         float64 `json:"FolateUg"`
	VitaminAUg       float64 `json:"VitaminAUg"`
	VitaminCMg       float64 `json:"VitaminCMg"`
	VitaminDIU       float64 `json:"VitaminDIU"`
	VitaminEMg       float64 `json:"VitaminEMg"`
	VitaminKUg       float64 `json:"VitaminKUg"`
	CalciumMg        float64 `json:"CalciumMg"`
	CopperMg         float64 `json:"CopperMg"`
	IronMg           float64 `json:"IronMg"`
	MagnesiumMg      float64 `json:"MagnesiumMg"`
	ManganeseMg      float64 `json:"ManganeseMg"`
	PhosphorusMg     float64 `json:"PhosphorusMg"`
	PotassiumMg      float64 `json:"PotassiumMg"`
	SeleniumUg       float64 `json:"SeleniumUg"`
	SodiumMg         float64 `json:"SodiumMg"`
	ZincMg           float64 `json:"ZincMg"`
	CholineMg        float64 `json:"CholineMg"`
}

// ServingRecords is a slice of ServingRecord.
type ServingRecords []ServingRecord

// ExerciseRecord is one row from the Cronometer "exercises" CSV export.
type ExerciseRecord struct {
	RecordedTime   time.Time `json:"RecordedTime"`
	Exercise       string    `json:"Exercise"`
	Minutes        float64   `json:"Minutes"`
	CaloriesBurned float64   `json:"CaloriesBurned"`
	Group          string    `json:"Group"`
}

// ExerciseRecords is a slice of ExerciseRecord.
type ExerciseRecords []ExerciseRecord

// BiometricRecord is one row from the Cronometer "biometrics" CSV export.
type BiometricRecord struct {
	RecordedTime time.Time `json:"RecordedTime"`
	Metric       string    `json:"Metric"`
	Amount       float64   `json:"Amount"`
	Unit         string    `json:"Unit"`
}

// BiometricRecords is a slice of BiometricRecord.
type BiometricRecords []BiometricRecord
