/*

build & run:

cd pkg/accel
g++ -g -std=c++17 -fopenmp -I. -o accel_test debug/accel_test.cpp accel.cpp -lgomp -lstdc++ && ./accel_test

*/

#include <float.h>
#include <stdio.h>
#include <string>
#include <vector>

int main(int argc, char** argv) {
	printf("Hello world\n");
	return 0;
}
