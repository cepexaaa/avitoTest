# Нагрузочное тестирование

#### Я использовал инструмент hey

Простые Get запросы к серверу:
```bash
hey -n 500 -c 20 "http://localhost:8080/team/get?team_name=developers"
```

Вывод:
```
Summary:
  Total:	0.0796 secs
  Slowest:	0.0534 secs
  Fastest:	0.0003 secs
  Average:	0.0030 secs
  Requests/sec:	6282.7022
  
  Total data:	99500 bytes
  Size/request:	199 bytes

Response time histogram:
  0.000 [1]	|
  0.006 [471]	|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.011 [7]	|■
  0.016 [2]	|
  0.022 [2]	|
  0.027 [1]	|
  0.032 [0]	|
  0.037 [1]	|
  0.043 [1]	|
  0.048 [10]	|■
  0.053 [4]	|


Latency distribution:
  10% in 0.0007 secs
  25% in 0.0009 secs
  50% in 0.0012 secs
  75% in 0.0017 secs
  90% in 0.0029 secs
  95% in 0.0072 secs
  99% in 0.0478 secs

Details (average, fastest, slowest):
  DNS+dialup:	0.0000 secs, 0.0003 secs, 0.0534 secs
  DNS-lookup:	0.0000 secs, 0.0000 secs, 0.0012 secs
  req write:	0.0000 secs, 0.0000 secs, 0.0008 secs
  resp wait:	0.0029 secs, 0.0003 secs, 0.0518 secs
  resp read:	0.0000 secs, 0.0000 secs, 0.0008 secs

Status code distribution:
  [200]	500 responses

```

Запрос создания PR:
```bash
hey -n 300 -c 10 -m POST -H "Content-Type: application/json" -d '{
    "pull_request_id": "load-test-{{.N}}",
    "pull_request_name": "Load Test PR",
    "author_id": "u1"
}' http://localhost:8080/pullRequest/create
```

Результат:
```
Summary:
  Total:	0.1683 secs
  Slowest:	0.0324 secs
  Fastest:	0.0008 secs
  Average:	0.0050 secs
  Requests/sec:	1782.5191
  
  Total data:	60865 bytes
  Size/request:	202 bytes

Response time histogram:
  0.001 [1]	|
  0.004 [185]	|■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.007 [53]	|■■■■■■■■■■■
  0.010 [26]	|■■■■■■
  0.013 [12]	|■■■
  0.017 [7]	|■■
  0.020 [5]	|■
  0.023 [4]	|■
  0.026 [2]	|
  0.029 [2]	|
  0.032 [3]	|■


Latency distribution:
  10% in 0.0013 secs
  25% in 0.0017 secs
  50% in 0.0029 secs
  75% in 0.0060 secs
  90% in 0.0115 secs
  95% in 0.0173 secs
  99% in 0.0296 secs

Details (average, fastest, slowest):
  DNS+dialup:	0.0000 secs, 0.0008 secs, 0.0324 secs
  DNS-lookup:	0.0000 secs, 0.0000 secs, 0.0012 secs
  req write:	0.0000 secs, 0.0000 secs, 0.0007 secs
  resp wait:	0.0049 secs, 0.0008 secs, 0.0308 secs
  resp read:	0.0000 secs, 0.0000 secs, 0.0002 secs

Status code distribution:
  [201]	300 responses

```

Это хорошие результаты для сервиса
