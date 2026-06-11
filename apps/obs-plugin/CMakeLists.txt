cmake_minimum_required(VERSION 3.16)

set(DELAYDECK_VERSION_FILE "${CMAKE_CURRENT_SOURCE_DIR}/../../VERSION")
if(NOT EXISTS "${DELAYDECK_VERSION_FILE}")
  message(FATAL_ERROR "VERSION file not found: ${DELAYDECK_VERSION_FILE}")
endif()

file(READ "${DELAYDECK_VERSION_FILE}" DELAYDECK_VERSION_RAW)
string(STRIP "${DELAYDECK_VERSION_RAW}" DELAYDECK_VERSION)
if(NOT DELAYDECK_VERSION MATCHES "^[0-9]+\\.[0-9]+\\.[0-9]+$")
  message(FATAL_ERROR "VERSION must be major.minor.patch, got: ${DELAYDECK_VERSION}")
endif()

project(delaydeck-plugin VERSION ${DELAYDECK_VERSION} LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

if(NOT CMAKE_PREFIX_PATH)
  message(FATAL_ERROR
    "CMAKE_PREFIX_PATH must point at an OBS Studio install or build tree.\n"
    "Example:\n"
    "  cmake -S apps/obs-plugin -B apps/obs-plugin/build "
    "-DCMAKE_PREFIX_PATH=\"C:/dev/obs-studio/build/install\"")
endif()

find_package(libobs REQUIRED)
find_package(obs-frontend-api REQUIRED)
find_package(Qt6 REQUIRED COMPONENTS Core Widgets Network)

set(PLUGIN_SOURCES
    src/plugin-module.c
    src/plugin-main.cpp
    src/config/dock-settings.cpp
    src/config/dock-settings.hpp
    src/config/credential-store-win.cpp
    src/config/credential-store.hpp
    src/config/relay-destination-url.cpp
    src/config/relay-destination-url.hpp
    src/config/relay-platform-presets.cpp
    src/config/relay-platform-presets.hpp
    src/config/relay-settings.cpp
    src/config/relay-settings.hpp
    src/config/setup-dialog.cpp
    src/config/setup-dialog.hpp
    src/dock/about-dialog.cpp
    src/dock/about-dialog.hpp
    src/dock/delaydeck-dock.cpp
    src/dock/delaydeck-dock.hpp
    src/obs/slate-scene-controller.cpp
    src/obs/slate-scene-controller.hpp
    src/obs/streaming-setup.cpp
    src/obs/streaming-setup.hpp
    src/relay/relay-client.cpp
    src/relay/relay-client.hpp
    src/relay/relay-process.cpp
    src/relay/relay-process.hpp
    src/relay/relay-event-socket.cpp
    src/relay/relay-event-socket.hpp
    src/relay/relay-types.hpp
    src/preflight/preflight-checker.cpp
    src/preflight/preflight-checker.hpp
    src/preflight/preflight-dialog.cpp
    src/preflight/preflight-dialog.hpp
    src/preflight/preflight-result.hpp
    src/preflight/streaming-guard.cpp
    src/preflight/streaming-guard.hpp
)

set(DELAYDECK_GENERATED_INCLUDE_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")
file(MAKE_DIRECTORY "${DELAYDECK_GENERATED_INCLUDE_DIR}")
configure_file(
  "${CMAKE_CURRENT_SOURCE_DIR}/src/version.hpp.in"
  "${DELAYDECK_GENERATED_INCLUDE_DIR}/version.hpp"
  @ONLY
)

add_library(delaydeck MODULE ${PLUGIN_SOURCES})

target_include_directories(delaydeck PRIVATE src "${DELAYDECK_GENERATED_INCLUDE_DIR}")

target_link_libraries(delaydeck PRIVATE
    OBS::libobs
    OBS::obs-frontend-api
    Qt6::Core
    Qt6::Widgets
    Qt6::Network
)

set_target_properties(delaydeck PROPERTIES
    PREFIX ""
    AUTOMOC ON
)

if(WIN32)
  set(OBS_PLUGIN_DEST "obs-plugins/64bit")
  set(OBS_DATA_DEST "data/obs-plugins/delaydeck")
  target_link_libraries(delaydeck PRIVATE advapi32 crypt32)
else()
  set(OBS_PLUGIN_DEST "lib/obs-plugins")
  set(OBS_DATA_DEST "share/obs/obs-plugins/delaydeck")
endif()

install(TARGETS delaydeck
    LIBRARY DESTINATION ${OBS_PLUGIN_DEST}
    RUNTIME DESTINATION ${OBS_PLUGIN_DEST}
)

install(DIRECTORY data/locale/
    DESTINATION ${OBS_DATA_DEST}/locale
)
