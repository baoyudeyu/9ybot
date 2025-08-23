-- PC28预测机器人数据库初始化脚本

-- 创建开奖数据表
CREATE TABLE IF NOT EXISTS lottery_results (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    qihao VARCHAR(20) UNIQUE NOT NULL COMMENT '期号',
    opentime DATETIME NOT NULL COMMENT '开奖时间',
    opennum VARCHAR(20) NOT NULL COMMENT '开奖号码',
    sum_value INT NOT NULL COMMENT '和值',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
    INDEX idx_qihao (qihao),
    INDEX idx_opentime (opentime),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='开奖数据表';

-- 创建预测记录表
CREATE TABLE IF NOT EXISTS predictions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    target_qihao VARCHAR(20) NOT NULL COMMENT '目标期号',
    predicted_num VARCHAR(20) NOT NULL COMMENT '预测号码',
    actual_num VARCHAR(20) DEFAULT NULL COMMENT '实际开奖号码',
    is_correct BOOLEAN DEFAULT NULL COMMENT '是否预测正确',
    confidence_score DECIMAL(5,2) DEFAULT NULL COMMENT '置信度评分',
    algorithm_version VARCHAR(50) DEFAULT 'default' COMMENT '算法版本',
    predicted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '预测时间',
    verified_at TIMESTAMP NULL COMMENT '验证时间',
    INDEX idx_target_qihao (target_qihao),
    INDEX idx_predicted_at (predicted_at),
    INDEX idx_is_correct (is_correct),
    INDEX idx_verified_at (verified_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='预测记录表';

-- 创建缓存状态表（用于跟踪缓存状态）
CREATE TABLE IF NOT EXISTS cache_status (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    cache_key VARCHAR(255) UNIQUE NOT NULL COMMENT '缓存键',
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
    data_version VARCHAR(50) NOT NULL COMMENT '数据版本',
    status ENUM('active', 'expired', 'invalid') DEFAULT 'active' COMMENT '缓存状态',
    INDEX idx_cache_key (cache_key),
    INDEX idx_last_updated (last_updated),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='缓存状态表';

-- 插入初始缓存状态记录
INSERT IGNORE INTO cache_status (cache_key, data_version, status) VALUES
('lottery:latest', 'v1.0', 'expired'),
('lottery:last3', 'v1.0', 'expired'),
('prediction:latest', 'v1.0', 'expired'),
('prediction:history:10', 'v1.0', 'expired'),
('stats:accuracy', 'v1.0', 'expired');

-- 创建数据清理存储过程
DELIMITER //
CREATE PROCEDURE IF NOT EXISTS CleanOldData()
BEGIN
    DECLARE done INT DEFAULT FALSE;
    DECLARE retention_hours INT DEFAULT 24;
    
    -- 清理超过24小时的开奖数据
    DELETE FROM lottery_results 
    WHERE created_at < DATE_SUB(NOW(), INTERVAL retention_hours HOUR);
    
    -- 清理超过24小时的预测记录
    DELETE FROM predictions 
    WHERE predicted_at < DATE_SUB(NOW(), INTERVAL retention_hours HOUR);
    
    -- 清理过期的缓存状态记录
    DELETE FROM cache_status 
    WHERE last_updated < DATE_SUB(NOW(), INTERVAL 2 HOUR) 
    AND status = 'expired';
    
    SELECT CONCAT('Cleaned data older than ', retention_hours, ' hours') as result;
END //
DELIMITER ;

-- 创建获取最新开奖数据的视图
CREATE OR REPLACE VIEW latest_lottery_view AS
SELECT * FROM lottery_results 
ORDER BY opentime DESC 
LIMIT 10;

-- 创建预测统计视图
CREATE OR REPLACE VIEW prediction_stats_view AS
SELECT 
    COUNT(*) as total_predictions,
    SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END) as correct_predictions,
    ROUND(
        (SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END) * 100.0 / COUNT(*)), 2
    ) as accuracy_rate,
    MIN(predicted_at) as first_prediction,
    MAX(predicted_at) as last_prediction
FROM predictions 
WHERE is_correct IS NOT NULL;

-- 创建索引优化查询性能
CREATE INDEX IF NOT EXISTS idx_lottery_opentime_desc ON lottery_results (opentime DESC);
CREATE INDEX IF NOT EXISTS idx_predictions_compound ON predictions (target_qihao, is_correct, predicted_at);

-- 显示表创建结果
SHOW TABLES;

