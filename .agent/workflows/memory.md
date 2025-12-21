---
description: 项目通用的规则
---

【后端】
** 当有新的配置产生时，默认更新env.example和env.docker，并解释含义
** 有mysql 新表或表变更时，更新schema.sql
** 绝对不允许在代码里有任何的DDL语句出现

【前端】
** 所有页面都使用统一的Element Plus组件
** 国际化：所有页面使用i18n翻译