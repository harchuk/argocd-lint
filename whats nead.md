Сделаем «argocd-lint» — линтер для Argo CD манифестов (Application/ApplicationSet): офлайн-проверки + опциональные «живые» валидации. В самом Argo CD команды argocd lint нет (есть лишь обсуждение флага валидации в CLI); зато у Argo Workflows есть argo lint, что подтверждает спрос на идею.  ￼

Что входит в MVP (реально выполнимо)
	•	Статическая схема-валидация CRD Application и ApplicationSet (встроенные JSON-схемы из CRD, без кластера). Опираемся на официальные спеки.  ￼
	•	Набор правил best-practice (вкл./выкл. через конфиг).
	•	Выводы: tab/JSON/SARIF (для GitHub Code Scanning).
	•	Интеграции: pre-commit hook и GitHub Action.
	•	Ноль сторонних зависимостей в рантайме (один бинарник).

Допы «чуть позже»: рендер Helm/Kustomize и kubeconform/--dry-run=server проверка против API (по аналогии с тем, что просят добавить в argocd CLI).  ￼

Правила (ядро)
	1.	targetRevision должен быть фиксирован: запрещаем HEAD, плавающие ветки/теги и маски v1.* (переключаемо).
	2.	spec.project ≠ default (требуем явный проект для RBAC/границ).
	3.	destination.namespace обязателен (если не кластер-scope).
	4.	syncPolicy: требуем явное решение — automated/manual; предупреждаем, если prune/selfHeal отключены в проектах, где положены.
	5.	Finalizer заявлен осознанно: если нет resources-finalizer.argocd.argoproj.io, сигналим «каскадное удаление не включено»; если есть — отмечаем как сознательное решение (информ-правило).  ￼
	6.	ignoreDifferences узкий: запрет на широкие маски/весь kind.
	7.	ApplicationSet: уникальность имён, отсутствие коллизий, и goTemplateOptions: ["missingkey=error"] по умолчанию.  ￼
	8.	Источник корректный: repoURL/path/chart согласованы; source vs sources не смешиваются.
	9.	Проектные гайдлайны: правила из статей по лучшим практикам (структура репо, app-of-apps и т.п.) как «advisory» уровень.  ￼

UX/CLI

# Локальная проверка каталога с манифестами
argocd-lint ./apps --apps --appsets --rules rules.yaml --format table

# «Строгий» режим: ошибка при любом warn>=medium
argocd-lint ./apps --severity-threshold=warn

# Проверка с рендером и API (опционально)
argocd-lint ./envs/prod --render --kubeconfig ~/.kube/config --dry-run=server

Конфиг правил (rules.yaml): включение/исключение, уровни (info|warn|error), override по пути/глобально.

Архитектура (лаконично)
	•	Язык: Go.
	•	Слои:
	1.	Parser (YAML→JSON, распознаёт Application/ApplicationSet).
	2.	Schema: встроенные JSON-схемы CRD + kubeconform-подобная проверка.
	3.	Rules: движок на простых предикатах (JSONPath) + опционально Rego (OPA) для сложных кейсов.
	4.	Render (не в MVP): Helm/Kustomize, затем серверный dry-run.
	•	Плагины: возможность запускать линтер внутри repo-server как plugin (для «policy-as-code» в кластере).  ￼

Интеграции
	•	pre-commit: - repo: local … entry: argocd-lint
	•	GitHub Action: job, который публикует SARIF в Security → Code scanning.
	•	PR-комментарии: краткий отчёт, ссылки на правило и фрагмент YAML.

Почему это зайдёт

— Заполняет пустоту: у Argo CD нет встроенного линтера, а потребность в офлайн-гейтах очевидна (даже фичреквесты указывают на валидацию).  ￼
— Есть «маячок» у соседей: argo lint в Workflows показывает полезность модели.  ￼

Надо сделать каркас репозитория (Go-модуль, Makefile, GitHub Action, 10 правил из списка, демо-набор манифестов) и первый релиз с бинарём под Linux/macOS/Windows.