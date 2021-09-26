// +build !ios

package gui

/*
#cgo CFLAGS: -pipe -O2 -arch x86_64 -isysroot /Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX10.15.sdk -mmacosx-version-min=10.12 -Wall -W -fPIC -DQT_NO_DEBUG -DQT_WIDGETS_LIB -DQT_GUI_LIB -DQT_CORE_LIB
#cgo CXXFLAGS: -pipe -stdlib=libc++ -O2 -std=gnu++11 -arch x86_64 -isysroot /Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX10.15.sdk -mmacosx-version-min=10.12 -Wall -W -fPIC -DQT_NO_DEBUG -DQT_WIDGETS_LIB -DQT_GUI_LIB -DQT_CORE_LIB
#cgo CXXFLAGS: -I../../src -I. -I/usr/local/Cellar/go/1.13.4/libexec/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib/QtWidgets.framework/Headers -I/usr/local/Cellar/go/1.13.4/libexec/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib/QtGui.framework/Headers -I/usr/local/Cellar/go/1.13.4/libexec/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib/QtCore.framework/Headers -I. -I/Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX10.15.sdk/System/Library/Frameworks/OpenGL.framework/Headers -I/Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX10.15.sdk/System/Library/Frameworks/AGL.framework/Headers -I/usr/local/Cellar/go/1.13.4/libexec/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/mkspecs/macx-clang -F/usr/local/Cellar/go/1.13.4/libexec/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib
#cgo LDFLAGS: -stdlib=libc++ -headerpad_max_install_names -arch x86_64 -Wl,-syslibroot,/Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX10.15.sdk -mmacosx-version-min=10.12  -Wl,-rpath,/usr/local/Cellar/go/1.13.4/libexec/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib
#cgo LDFLAGS:  -F/usr/local/Cellar/go/1.13.4/libexec/src/github.com/therecipe/env_darwin_amd64_513/5.13.0/clang_64/lib -framework QtWidgets -framework QtGui -framework QtCore -framework OpenGL -framework AGL
#cgo CFLAGS: -Wno-unused-parameter -Wno-unused-variable -Wno-return-type
#cgo CXXFLAGS: -Wno-unused-parameter -Wno-unused-variable -Wno-return-type
*/
import "C"
