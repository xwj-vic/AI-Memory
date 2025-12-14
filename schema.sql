-- 1. Create Database (Must be done manually or ensure it exists)
CREATE DATABASE IF NOT EXISTS ai_memory;

-- 2. Use the Database
USE ai_memory;

-- 3. Create Users Table (handled by app auto-migration, but here for reference)
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 4. Initial Admin User (Optional, app creates this on startup if missing)
-- Password: admin123
INSERT INTO users (username, password_hash) VALUES ('admin', '$2a$10$FrMJpuNsOfEY.5edOFDoSOWsswfLVPG.MutG8xNcXtcZrc75YyYxu') ON DUPLICATE KEY UPDATE id=id;

-- 5. End Users Table (Tracks users interacting with the AI)
CREATE TABLE IF NOT EXISTS end_users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_identifier VARCHAR(255) NOT NULL UNIQUE,
    last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

