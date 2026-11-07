# Источники состояния

Users/tasks сохраняются в PostgreSQL после API restart. Sessions сохраняются в Redis
после API restart, но теряются при Redis restart без persistence. Это разные опыты.
Процедура перехода: [setup](setup.md). Полный flow: [demo](demo.md).
