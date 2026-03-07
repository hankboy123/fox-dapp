package models

var allModels []interface{}

func Register(model interface{}) {
	allModels = append(allModels, model)
}

func GetModels() []interface{} {
	Register(&User{})
	Register(&Comment{})
	Register(&Post{})
	return allModels
}
