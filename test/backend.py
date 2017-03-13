from wsgiref.util import request_uri
def application(environ, start_response):
    print("Request %s\n" % request_uri(environ))
    start_response('200 OK', [('Content-Type', 'text/plain')])
    return ["Hello World\n%s\n" % request_uri(environ)]
