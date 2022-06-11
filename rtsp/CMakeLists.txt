cmake_minimum_required(VERSION 3.22)
set(CMAKE_CXX_STANDARD 14)
set(CMAKE_CXX_STANDARD_REQUIRED True)
project(rtsp)

# It looks like live555 doesn't provide a CMake library, so we need to source it manually
#find_package(live555 CONFIG REQUIRED)  # This doesn't work

# Source live555 manually
find_path(LIVE555_INCLUDE_DIR liveMedia.hh)
#find_library(LIVE555_LIBRARIES liveMedia groupsock BasicUsageEnvironment UsageEnvironment)     # This doesn't work
find_library(LIVE555_LIBRARY1 liveMedia)
find_library(LIVE555_LIBRARY2 groupsock)
find_library(LIVE555_LIBRARY3 BasicUsageEnvironment)
find_library(LIVE555_LIBRARY4 UsageEnvironment)

find_package(OpenSSL REQUIRED)

#add_executable(rtsp hello.cpp)
add_executable(rtsp testRTSPClient.cpp)
target_include_directories(rtsp PRIVATE ${LIVE555_INCLUDE_DIR})
target_link_libraries(rtsp PRIVATE ${LIVE555_LIBRARY1} ${LIVE555_LIBRARY2} ${LIVE555_LIBRARY3} ${LIVE555_LIBRARY4})
target_link_libraries(rtsp PRIVATE OpenSSL::SSL OpenSSL::Crypto)