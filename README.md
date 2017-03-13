caddy-uwsgi
===========

`Caddy` plugin that enables basic `uwsgi` protocol support.


Caddyfile
---------
*test/Caddyfile*
```
:80 {
	uwsgi / localhost:8080
	errors stdout
	log stdout
}
```

uWSGI
-----
*test/run.sh*
```
#/bin/bash
uwsgi --socket=:8080 --wsgi=backend
```

Python
------
*test/backend.py*
```py
from wsgiref.util import request_uri
def application(environ, start_response):
    print("Request %s\n" % request_uri(environ))
    start_response('200 OK', [('Content-Type', 'text/plain')])
    return ["Hello World\n%s\n" % request_uri(environ)]
```
