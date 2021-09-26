// +build !ios

package gui

/*
#cgo CFLAGS: -pipe -O2 -arch x86_64 -isysroot /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk -mmacosx-version-min=10.12 -Wall -W -fPIC -DQT_NO_DEBUG -DQT_WIDGETS_LIB -DQT_GUI_LIB -DQT_QML_LIB -DQT_NETWORK_LIB -DQT_CORE_LIB
#cgo CXXFLAGS: -pipe -stdlib=libc++ -O2 -std=gnu++11 -arch x86_64 -isysroot /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk -mmacosx-version-min=10.12 -Wall -W -fPIC -DQT_NO_DEBUG -DQT_WIDGETS_LIB -DQT_GUI_LIB -DQT_QML_LIB -DQT_NETWORK_LIB -DQT_CORE_LIB
#cgo CXXFLAGS: -I../../src -I. -I../github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib/QtWidgets.framework/Headers -I../github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib/QtGui.framework/Headers -I../github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib/QtQml.framework/Headers -I../github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib/QtNetwork.framework/Headers -I../github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib/QtCore.framework/Headers -I. -I/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/System/Library/Frameworks/OpenGL.framework/Headers -I/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/System/Library/Frameworks/AGL.framework/Headers -I../github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/mkspecs/macx-clang -F/Users/rigensen/workspace/mpegps-parser/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib
#cgo LDFLAGS: -stdlib=libc++ -headerpad_max_install_names -arch x86_64 -Wl,-syslibroot,/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk -mmacosx-version-min=10.12  -Wl,-rpath,/Users/rigensen/workspace/mpegps-parser/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib
#cgo LDFLAGS:  -F/Users/rigensen/workspace/mpegps-parser/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib -framework QtWidgets -framework QtGui -framework QtQml -framework QtNetwork -framework QtCore -framework OpenGL -framework AGL
#cgo CFLAGS: -Wno-unused-parameter -Wno-unused-variable -Wno-return-type
#cgo CXXFLAGS: -Wno-unused-parameter -Wno-unused-variable -Wno-return-type
*/
import "C"
