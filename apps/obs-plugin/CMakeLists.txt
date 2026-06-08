cmake_minimum_required(VERSION 3.16)

project(delaydeck-plugin VERSION 0.1.0 LANGUAGES CXX)

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
    src/plugin-main.cpp
    src/dock/delaydeck-dock.cpp
    src/dock/delaydeck-dock.hpp
    src/relay/relay-client.cpp
    src/relay/relay-client.hpp
    src/relay/relay-process.cpp
    src/relay/relay-process.hpp
    src/relay/relay-event-socket.cpp
    src/relay/relay-event-socket.hpp
    src/relay/relay-types.hpp
)

add_library(delaydeck MODULE ${PLUGIN_SOURCES})

target_include_directories(delaydeck PRIVATE src)

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
