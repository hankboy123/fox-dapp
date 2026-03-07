package services

import (
	"backend/models"
	"backend/utils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService struct {
	// 这里可以添加数据库连接等依赖
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// 在这里添加用户相关的方法，例如创建用户、获取用户信息等
func (s *UserService) CreateUser(req models.CreateUserRequest) (*models.User, error) {
	var existingUser models.User
	if err := s.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		return nil, utils.NewAppError(409, "Username already exists")
	}

	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, utils.NewAppError(409, "Email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, utils.NewAppError(500, "Failed to hash password")
	}

	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to create user")
	}

	return user, nil
}
func (s *UserService) GetUserByID(userID uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, utils.NewAppError(404, "User not found")
		}
		return nil, utils.NewAppError(500, "Failed to retrieve user")
	}
	return &user, nil
}

func (s *UserService) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, utils.NewAppError(404, "User not found")
		}
		return nil, utils.NewAppError(500, "Failed to retrieve user")
	}
	return &user, nil
}

func (s *UserService) GetUserByName(username string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, utils.NewAppError(404, "User not found")
		}
		return nil, utils.NewAppError(500, "Failed to retrieve user")
	}
	return &user, nil
}

func (s *UserService) UpdateUser(userID uint, req models.UpdateUserRequest) (*models.User, error) {

	user, err := s.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, utils.NewAppError(500, "Failed to hash password")
		}
		user.Password = string(hashedPassword)
	}

	if err := s.db.Save(user).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to update user")
	}

	return user, nil
}
func (s *UserService) DeleteUser(userID uint) error {
	if err := s.db.Delete(&models.User{}, userID).Error; err != nil {
		return utils.NewAppError(500, "Failed to delete user")
	}
	return nil
}

func (s *UserService) Authenticate(username, password string) (*models.User, error) {
	user, err := s.GetUserByName(username)
	if err != nil {
		return nil, utils.NewAppError(401, "Invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, utils.NewAppError(401, "Invalid username or password")
	}

	return user, nil
}
