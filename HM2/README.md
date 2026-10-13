# Микросервисы product-service и cart

Два сервиса на Go: каталог товаров предоставляет gRPC API, корзина предоставляет HTTP API и при добавлении товара обращается к каталогу по gRPC. Состояние корзин находится в памяти. Каталог загружается из `products.json` при запуске.

## Запуск

Нужны Go 1.26+ и GNU Make. Из корня проекта:

```sh
make run-all
```

Каталог слушает порт `50051`, корзина — `8080`. Для генерации protobuf-кода нужны `protoc`, `protoc-gen-go` и `protoc-gen-go-grpc` в `PATH`:

```sh
make generate
```

Сгенерированный Go-код уже включён в проект, поэтому для обычной сборки `protoc` не требуется. Для проверки:

```sh
make test
```

Если GNU Make недоступен, запустите `go run ./cmd/product-service` и `go run ./cmd/cart` в двух терминалах из корня проекта.

## Примеры HTTP

```sh
curl -X POST http://localhost:8080/user/1007/cart/2958025 \
  -H 'Content-Type: application/json' -d '{"count":2}'
curl http://localhost:8080/user/1007/cart
```

POST также принимает JSON-число `2` как тело запроса. Повторное добавление одного SKU увеличивает `count`. GET возвращает товары по возрастанию SKU, либо 404 для отсутствующей/пустой корзины.

Документация HTTP API: [Swagger UI](http://localhost:8080/swagger/) и [OpenAPI JSON](http://localhost:8080/swagger.json). Интерфейс Swagger UI загружает свои файлы с CDN; сам контракт отдаётся сервисом локально.

## Настройки

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `PRODUCTS_FILE` | `products.json` | Файл каталога |
| `PRODUCT_GRPC_ADDR` | `:50051` | Адрес прослушивания product-service |
| `PRODUCT_GRPC_TARGET` | `localhost:50051` | Адрес каталога для cart |
| `CART_HTTP_ADDR` | `:8080` | Адрес прослушивания cart |

Контракт gRPC находится в `api/proto/product.proto`. `GetProduct` возвращает `InvalidArgument` для неположительного SKU и `NotFound` для отсутствующего товара. `ListProducts` принимает номер страницы `page` с 1 и размер `page_size` от 1 до 100; нулевые значения означают 1 и 20. Пустая страница возвращает пустой список, а `next_page = 0` означает конец каталога.
