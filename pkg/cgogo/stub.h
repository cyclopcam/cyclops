#include <malloc.h>
#include <string.h>

char* Foo() {
	char* buf = (char*) malloc(20);
	buf[0]    = 'a';
	buf[1]    = 'b';
	buf[2]    = 0;
	return buf;
}