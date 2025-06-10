cmake_minimum_required(VERSION 3.10.2)

project("cynative")

add_library(
	cynative
	SHARED
	src/main/cpp/native-lib.cpp
)

target_include_directories(cynative
	PRIVATE
	${CMAKE_CURRENT_SOURCE_DIR}/third_party/libyuv/include
)

add_subdirectory(third_party/libyuv)
target_link_libraries(cynative yuv log)