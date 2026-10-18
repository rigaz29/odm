cmake_minimum_required(VERSION 3.21)
project(odm-libtorrent LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
# The static C runtime on Windows, as the x64-windows-static triplet builds
# libtorrent and its dependencies with.
set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")

set(ODM_LIBTORRENT_VERSION "dev" CACHE STRING "The version odm-libtorrent reports")

find_package(LibtorrentRasterbar CONFIG REQUIRED)
find_package(nlohmann_json CONFIG REQUIRED)

add_executable(odm-libtorrent
  src/main.cpp
  src/helper.cpp
  src/create.cpp
  src/util.cpp
)
target_compile_definitions(odm-libtorrent PRIVATE "ODM_LIBTORRENT_VERSION=\"${ODM_LIBTORRENT_VERSION}\"")
target_link_libraries(odm-libtorrent PRIVATE
  LibtorrentRasterbar::torrent-rasterbar
  nlohmann_json::nlohmann_json
)
if(WIN32)
  target_compile_definitions(odm-libtorrent PRIVATE NOMINMAX)
  target_link_libraries(odm-libtorrent PRIVATE shell32)
endif()
if(MSVC)
  target_compile_options(odm-libtorrent PRIVATE /utf-8)
endif()
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
  # One program for every distribution: only glibc stays shared.
  target_link_options(odm-libtorrent PRIVATE -static-libstdc++ -static-libgcc)
endif()
