package auth

import (
    "crypto/rand"
    "crypto/subtle"
    "database/sql"
    "encoding/base64"
    "fmt"
    "strings"
    "time"

    "github.com/casapps/casspaces/internal/database"
    "github.com/casapps/casspaces/internal/utils"
    "github.com/golang-jwt/jwt/v5"
    "github.com/pkg/errors"
    "github.com/sirupsen/logrus"
    "golang.org/x/crypto/argon2"
)

type Service struct {
    db           database.Database
    logger       *logrus.Logger
    config       *Config
    jwtSecret    []byte
    sessions     map[string]*Session
}

type Config struct {
    MinPasswordLength     int           `json:"min_password_length"`
    RequireUppercase      bool          `json:"require_uppercase"`
    RequireLowercase      bool          `json:"require_lowercase"`
    RequireNumbers        bool          `json:"require_numbers"`
    RequireSpecialChars   bool          `json:"require_special_chars"`
    SessionTimeout        time.Duration `json:"session_timeout"`
    MaxConcurrentSessions int           `json:"max_concurrent_sessions"`
    MaxFailedLogins       int           `json:"max_failed_logins"`
    LockoutDuration       time.Duration `json:"lockout_duration"`
    TwoFactorRequired     bool          `json:"two_factor_required"`
    JWTExpirationTime     time.Duration `json:"jwt_expiration_time"`
    JWTIssuer            string        `json:"jwt_issuer"`
}

type User struct {
    ID                    int       `json:"id" db:"id"`
    Username              string    `json:"username" db:"username"`
    Email                 string    `json:"email" db:"email"`
    PasswordHash          string    `json:"-" db:"password_hash"`
    FullName              string    `json:"full_name" db:"full_name"`
    Active                bool      `json:"active" db:"active"`
    IsAdmin               bool      `json:"is_admin" db:"is_admin"`
    IsTempAdmin           bool      `json:"is_temp_admin" db:"is_temp_admin"`
    EmailVerified         bool      `json:"email_verified" db:"email_verified"`
    FailedLoginAttempts   int       `json:"-" db:"failed_login_attempts"`
    LockedUntil           *time.Time `json:"-" db:"locked_until"`
    LastLogin             *time.Time `json:"last_login" db:"last_login"`
    LastLoginIP           string    `json:"last_login_ip" db:"last_login_ip"`
    PasswordChangedAt     time.Time `json:"-" db:"password_changed_at"`
    TwoFactorEnabled      bool      `json:"two_factor_enabled" db:"two_factor_enabled"`
    TwoFactorSecret       string    `json:"-" db:"two_factor_secret"`
    TwoFactorBackupCodes  string    `json:"-" db:"two_factor_backup_codes"`
    WorkspaceQuota        int       `json:"workspace_quota" db:"workspace_quota"`
    StorageQuotaGB        int       `json:"storage_quota_gb" db:"storage_quota_gb"`
    CreatedAt             time.Time `json:"created_at" db:"created_at"`
    UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

type Session struct {
    ID           string    `json:"id" db:"id"`
    UserID       int       `json:"user_id" db:"user_id"`
    IPAddress    string    `json:"ip_address" db:"ip_address"`
    UserAgent    string    `json:"user_agent" db:"user_agent"`
    Active       bool      `json:"active" db:"active"`
    ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
    LastActivity time.Time `json:"last_activity" db:"last_activity"`
}

type LoginRequest struct {
    Username   string `json:"username" validate:"required"`
    Password   string `json:"password" validate:"required"`
    TwoFactor  string `json:"two_factor,omitempty"`
    RememberMe bool   `json:"remember_me"`
}

type LoginResponse struct {
    Success      bool   `json:"success"`
    Message      string `json:"message"`
    Token        string `json:"token,omitempty"`
    SessionID    string `json:"session_id,omitempty"`
    User         *User  `json:"user,omitempty"`
    RequiresTwoFactor bool `json:"requires_two_factor"`
}

type Claims struct {
    UserID    int    `json:"user_id"`
    Username  string `json:"username"`
    IsAdmin   bool   `json:"is_admin"`
    SessionID string `json:"session_id"`
    jwt.RegisteredClaims
}

func New(db database.Database, securityConfig interface{}, logger *logrus.Logger) (*Service, error) {
    config := &Config{
        MinPasswordLength:     12,
        RequireUppercase:      true,
        RequireLowercase:      true,
        RequireNumbers:        true,
        RequireSpecialChars:   true,
        SessionTimeout:        24 * time.Hour,
        MaxConcurrentSessions: 5,
        MaxFailedLogins:       5,
        LockoutDuration:       15 * time.Minute,
        TwoFactorRequired:     false,
        JWTExpirationTime:     1 * time.Hour,
        JWTIssuer:            "casspaces",
    }

    // Generate JWT secret
    jwtSecret := make([]byte, 32)
    if _, err := rand.Read(jwtSecret); err != nil {
        return nil, errors.Wrap(err, "failed to generate JWT secret")
    }

    service := &Service{
        db:        db,
        logger:    logger,
        config:    config,
        jwtSecret: jwtSecret,
        sessions:  make(map[string]*Session),
    }

    logger.Info("✅ Authentication service initialized")
    return service, nil
}

func (s *Service) CreateUser(username, email, password, fullName string, isAdmin bool) (*User, error) {
    // Validate input
    if err := s.validatePassword(password); err != nil {
        return nil, errors.Wrap(err, "password validation failed")
    }

    if err := s.validateUsername(username); err != nil {
        return nil, errors.Wrap(err, "username validation failed")
    }

    if err := s.validateEmail(email); err != nil {
        return nil, errors.Wrap(err, "email validation failed")
    }

    // Check if user already exists
    if s.userExists(username, email) {
        return nil, errors.New("user with this username or email already exists")
    }

    // Hash password
    passwordHash, err := s.hashPassword(password)
    if err != nil {
        return nil, errors.Wrap(err, "failed to hash password")
    }

    // Create user
    result, err := s.db.Exec(`
        INSERT INTO users (
            username, email, password_hash, full_name, is_admin,
            active, email_verified, workspace_quota, storage_quota_gb,
            password_changed_at
        ) VALUES (?, ?, ?, ?, ?, TRUE, FALSE, 10, 10, CURRENT_TIMESTAMP)
    `, username, email, passwordHash, fullName, isAdmin)

    if err != nil {
        return nil, errors.Wrap(err, "failed to create user")
    }

    userID, err := result.LastInsertId()
    if err != nil {
        return nil, errors.Wrap(err, "failed to get user ID")
    }

    // Fetch created user
    user, err := s.GetUserByID(int(userID))
    if err != nil {
        return nil, errors.Wrap(err, "failed to fetch created user")
    }

    s.logger.Infof("✅ Created user: %s (ID: %d)", username, userID)
    return user, nil
}

func (s *Service) Authenticate(req *LoginRequest, ipAddress, userAgent string) (*LoginResponse, error) {
    // Get user
    user, err := s.GetUserByUsername(req.Username)
    if err != nil {
        return &LoginResponse{
            Success: false,
            Message: "Invalid username or password",
        }, nil
    }

    // Check if user is active
    if !user.Active {
        return &LoginResponse{
            Success: false,
            Message: "Account is disabled",
        }, nil
    }

    // Verify password
    if !s.verifyPassword(req.Password, user.PasswordHash) {
        return &LoginResponse{
            Success: false,
            Message: "Invalid username or password",
        }, nil
    }

    // Create session
    session, err := s.createSession(user, ipAddress, userAgent, req.RememberMe)
    if err != nil {
        return nil, errors.Wrap(err, "failed to create session")
    }

    // Generate JWT token
    token, err := s.generateJWT(user, session.ID)
    if err != nil {
        return nil, errors.Wrap(err, "failed to generate token")
    }

    s.logger.Infof("✅ User %s logged in from %s", user.Username, ipAddress)

    return &LoginResponse{
        Success:   true,
        Message:   "Login successful",
        Token:     token,
        SessionID: session.ID,
        User:      user,
    }, nil
}

func (s *Service) ValidateSession(sessionID string) (*User, error) {
    session, exists := s.sessions[sessionID]
    if !exists {
        return nil, errors.New("session not found")
    }

    // Check if session is expired
    if time.Now().After(session.ExpiresAt) {
        s.terminateSession(sessionID)
        return nil, errors.New("session expired")
    }

    // Get user
    user, err := s.GetUserByID(session.UserID)
    if err != nil {
        return nil, errors.Wrap(err, "failed to get user")
    }

    // Update last activity
    session.LastActivity = time.Now()

    return user, nil
}

func (s *Service) ValidateJWT(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return s.jwtSecret, nil
    })

    if err != nil {
        return nil, errors.Wrap(err, "failed to parse token")
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, errors.New("invalid token")
}

func (s *Service) GetUserByID(id int) (*User, error) {
    user := &User{}
    err := s.db.QueryRow(`
        SELECT id, username, email, password_hash, full_name, active,
               is_admin, is_temp_admin, email_verified, failed_login_attempts,
               locked_until, last_login, last_login_ip, password_changed_at,
               two_factor_enabled, two_factor_secret, two_factor_backup_codes,
               workspace_quota, storage_quota_gb, created_at, updated_at
        FROM users WHERE id = ?
    `, id).Scan(
        &user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.FullName,
        &user.Active, &user.IsAdmin, &user.IsTempAdmin, &user.EmailVerified,
        &user.FailedLoginAttempts, &user.LockedUntil, &user.LastLogin,
        &user.LastLoginIP, &user.PasswordChangedAt, &user.TwoFactorEnabled,
        &user.TwoFactorSecret, &user.TwoFactorBackupCodes, &user.WorkspaceQuota,
        &user.StorageQuotaGB, &user.CreatedAt, &user.UpdatedAt,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("user not found")
        }
        return nil, errors.Wrap(err, "database error")
    }

    return user, nil
}

func (s *Service) GetUserByUsername(username string) (*User, error) {
    user := &User{}
    err := s.db.QueryRow(`
        SELECT id, username, email, password_hash, full_name, active,
               is_admin, is_temp_admin, email_verified, failed_login_attempts,
               locked_until, last_login, last_login_ip, password_changed_at,
               two_factor_enabled, two_factor_secret, two_factor_backup_codes,
               workspace_quota, storage_quota_gb, created_at, updated_at
        FROM users WHERE username = ? OR email = ?
    `, username, username).Scan(
        &user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.FullName,
        &user.Active, &user.IsAdmin, &user.IsTempAdmin, &user.EmailVerified,
        &user.FailedLoginAttempts, &user.LockedUntil, &user.LastLogin,
        &user.LastLoginIP, &user.PasswordChangedAt, &user.TwoFactorEnabled,
        &user.TwoFactorSecret, &user.TwoFactorBackupCodes, &user.WorkspaceQuota,
        &user.StorageQuotaGB, &user.CreatedAt, &user.UpdatedAt,
    )

    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("user not found")
        }
        return nil, errors.Wrap(err, "database error")
    }

    return user, nil
}

func (s *Service) validatePassword(password string) error {
    if len(password) < s.config.MinPasswordLength {
        return fmt.Errorf("password must be at least %d characters long", s.config.MinPasswordLength)
    }

    if s.config.RequireUppercase && !utils.ContainsUppercase(password) {
        return errors.New("password must contain at least one uppercase letter")
    }

    if s.config.RequireLowercase && !utils.ContainsLowercase(password) {
        return errors.New("password must contain at least one lowercase letter")
    }

    if s.config.RequireNumbers && !utils.ContainsNumber(password) {
        return errors.New("password must contain at least one number")
    }

    if s.config.RequireSpecialChars && !utils.ContainsSpecialChar(password) {
        return errors.New("password must contain at least one special character")
    }

    return nil
}

func (s *Service) validateUsername(username string) error {
    if len(username) < 3 {
        return errors.New("username must be at least 3 characters long")
    }

    if len(username) > 50 {
        return errors.New("username must be less than 50 characters")
    }

    if !utils.IsValidUsername(username) {
        return errors.New("username contains invalid characters")
    }

    return nil
}

func (s *Service) validateEmail(email string) error {
    if !utils.IsValidEmail(email) {
        return errors.New("invalid email format")
    }

    return nil
}

func (s *Service) userExists(username, email string) bool {
    var count int
    err := s.db.QueryRow(`
        SELECT COUNT(*) FROM users
        WHERE username = ? OR email = ?
    `, username, email).Scan(&count)

    return err == nil && count > 0
}

func (s *Service) hashPassword(password string) (string, error) {
    salt := make([]byte, 16)
    if _, err := rand.Read(salt); err != nil {
        return "", err
    }

    hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

    // Encode salt and hash
    encoded := base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash)
    return encoded, nil
}

func (s *Service) verifyPassword(password, encodedHash string) bool {
    parts := strings.Split(encodedHash, "$")
    if len(parts) != 2 {
        return false
    }

    salt, err := base64.RawStdEncoding.DecodeString(parts[0])
    if err != nil {
        return false
    }

    expectedHash, err := base64.RawStdEncoding.DecodeString(parts[1])
    if err != nil {
        return false
    }

    actualHash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

    return subtle.ConstantTimeCompare(expectedHash, actualHash) == 1
}

func (s *Service) createSession(user *User, ipAddress, userAgent string, rememberMe bool) (*Session, error) {
    // Generate session ID
    sessionID, err := utils.GenerateSecureID(32)
    if err != nil {
        return nil, err
    }

    // Calculate expiration
    expiresAt := time.Now().Add(s.config.SessionTimeout)
    if rememberMe {
        expiresAt = time.Now().Add(30 * 24 * time.Hour) // 30 days
    }

    session := &Session{
        ID:           sessionID,
        UserID:       user.ID,
        IPAddress:    ipAddress,
        UserAgent:    userAgent,
        Active:       true,
        ExpiresAt:    expiresAt,
        CreatedAt:    time.Now(),
        LastActivity: time.Now(),
    }

    // Store in database
    _, err = s.db.Exec(`
        INSERT INTO user_sessions (
            id, user_id, ip_address, user_agent, active,
            expires_at, created_at, last_activity
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `, session.ID, session.UserID, session.IPAddress, session.UserAgent,
        session.Active, session.ExpiresAt, session.CreatedAt, session.LastActivity)

    if err != nil {
        return nil, errors.Wrap(err, "failed to create session")
    }

    // Store in memory
    s.sessions[sessionID] = session

    return session, nil
}

func (s *Service) terminateSession(sessionID string) {
    // Remove from memory
    delete(s.sessions, sessionID)

    // Mark as inactive in database
    _, err := s.db.Exec(`
        UPDATE user_sessions
        SET active = FALSE
        WHERE id = ?
    `, sessionID)

    if err != nil {
        s.logger.WithError(err).Error("Failed to terminate session in database")
    }
}

func (s *Service) generateJWT(user *User, sessionID string) (string, error) {
    claims := &Claims{
        UserID:    user.ID,
        Username:  user.Username,
        IsAdmin:   user.IsAdmin,
        SessionID: sessionID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.JWTExpirationTime)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            NotBefore: jwt.NewNumericDate(time.Now()),
            Issuer:    s.config.JWTIssuer,
            Subject:   user.Username,
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.jwtSecret)
}