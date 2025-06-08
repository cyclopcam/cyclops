cmake_minimum_required(VERSION 3.10.2)

project("your_native_lib")

add_library(
	your_native_lib
	SHARED
	src/main/cpp/native-lib.cpp
)

target_include_directories(your_native_lib
	PRIVATE
	${CMAKE_CURRENT_SOURCE_DIR}/third_party/libyuv/include
)

add_subdirectory(third_party/libyuv)
target_link_libraries(your_native_lib yuv log)