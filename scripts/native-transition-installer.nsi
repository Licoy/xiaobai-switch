; Native payload, compatible with tauri-plugin-updater /UPDATE /ARGS /P /S /R.
; Registry/path conventions follow Tauri's currentUser installer, not MyGo's default.
Unicode true
ManifestDPIAware true
RequestExecutionLevel user
SetCompressor /SOLID lzma
!include MUI2.nsh
!include FileFunc.nsh
!include LogicLib.nsh
!include x64.nsh
!include StrFunc.nsh
${StrStr}
!define PRODUCT "XiaoBaiSwitch"
!define BUNDLE "com.github.licoy.xiaobai-switch.app"
!ifndef SHORTCUT
!define SHORTCUT "$SMPROGRAMS\XiaoBaiSwitch.lnk"
!endif
!ifndef UNINSTALL
!define UNINSTALL "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT}"
!endif
!ifndef MANU
!define MANU "Software\github\${PRODUCT}"
!endif
!ifndef PROTOCOL
!define PROTOCOL "xiaobaiswitch"
!endif
!ifdef UNINSTALLSIGN
!uninstfinalize 'node "%NATIVE_SIGN_SCRIPT%" "%1"' = 0
!endif
Name "${PRODUCT}"
OutFile "${OUTPUT}"
InstallDir "$LOCALAPPDATA\${PRODUCT}"
VIProductVersion "${FILEVERSION}"
VIAddVersionKey "ProductName" "${PRODUCT}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "${PRODUCT} Installer"
VIAddVersionKey "LegalCopyright" "MIT"
Var UpdateMode
Var Passive
Var Restart
Var LaunchArgs
Var HadPrevious
Var MsiProduct
Var MsiLocation
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_FUNCTION Launch
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE English
!macro FindMsi ROOT VIEW
  ${If} $MsiProduct == ""
    SetRegView ${VIEW}
    StrCpy $1 0
    ${Do}
      EnumRegKey $2 ${ROOT} "Software\Microsoft\Windows\CurrentVersion\Uninstall" $1
      ${If} $2 == ""
        ${ExitDo}
      ${EndIf}
      IntOp $1 $1 + 1
      ReadRegStr $3 ${ROOT} "Software\Microsoft\Windows\CurrentVersion\Uninstall\$2" "DisplayName"
      ReadRegStr $4 ${ROOT} "Software\Microsoft\Windows\CurrentVersion\Uninstall\$2" "Publisher"
      ReadRegDWORD $5 ${ROOT} "Software\Microsoft\Windows\CurrentVersion\Uninstall\$2" "WindowsInstaller"
      ${If} $3 == "${PRODUCT}"
      ${AndIf} $4 == "github"
      ${AndIf} $5 == 1
        StrCpy $MsiProduct $2
        ReadRegStr $MsiLocation ${ROOT} "Software\Microsoft\Windows\CurrentVersion\Uninstall\$2" "InstallLocation"
        ${ExitDo}
      ${EndIf}
    ${Loop}
  ${EndIf}
!macroend
Function .onInit
  SetRegView 64
  ${GetOptions} $CMDLINE "/UPDATE" $0
  ${IfNot} ${Errors}
    StrCpy $UpdateMode 1
  ${EndIf}
  ${GetOptions} $CMDLINE "/P" $0
  ${IfNot} ${Errors}
    StrCpy $Passive 1
    SetSilent silent
  ${EndIf}
  ${GetOptions} $CMDLINE "/R" $0
  ${IfNot} ${Errors}
    StrCpy $Restart 1
  ${EndIf}
  ${GetOptions} $CMDLINE "/ARGS" $LaunchArgs
  ReadRegStr $0 HKCU "${MANU}" ""
  ${If} $0 == ""
    ReadRegStr $0 HKCU "${UNINSTALL}" "InstallLocation"
  ${EndIf}
  ${If} $0 != ""
    StrCpy $INSTDIR $0
  ${EndIf}
  !ifndef FIXTURE
  !insertmacro FindMsi HKCU 64
  !insertmacro FindMsi HKLM 64
  !insertmacro FindMsi HKCU 32
  !insertmacro FindMsi HKLM 32
  !endif
  SetRegView 64
  ${If} $MsiProduct != ""
    ; An MSI may live in Program Files; stage into a writable user directory first.
    StrCpy $INSTDIR "$LOCALAPPDATA\${PRODUCT}"
    ${If} $MsiLocation == $INSTDIR
      StrCpy $INSTDIR "$LOCALAPPDATA\${PRODUCT}-native"
    ${EndIf}
  ${EndIf}
FunctionEnd
Section Install
  InitPluginsDir
  SetOutPath "$PLUGINSDIR\payload"
  File /r "${PAYLOAD}\*"
  ${If} ${FileExists} "$INSTDIR.previous\*"
    Abort "A previous installation recovery directory exists, resolve it before installing"
  ${EndIf}
  StrCpy $HadPrevious 0
  ${If} ${FileExists} "$INSTDIR\*"
    ; The updater exits its parent after launching us, wait at most 30 seconds for release.
    StrCpy $1 0
    wait_parent:
      ClearErrors
      Rename "$INSTDIR" "$INSTDIR.previous"
      ${If} ${Errors}
        IntOp $1 $1 + 1
        ${If} $1 >= 30
          Abort "Application files are in use, close XiaoBaiSwitch and retry"
        ${EndIf}
        Sleep 1000
        Goto wait_parent
      ${EndIf}
    StrCpy $HadPrevious 1
  ${EndIf}
  ClearErrors
  CreateDirectory "$INSTDIR"
  CopyFiles /SILENT "$PLUGINSDIR\payload\*.*" "$INSTDIR"
  !ifdef FAIL_AFTER_COPY
    SetErrors
  !endif
  ${If} ${Errors}
    RMDir /r "$INSTDIR"
    ${If} $HadPrevious == 1
      ClearErrors
      Rename "$INSTDIR.previous" "$INSTDIR"
      ${If} ${Errors}
        Abort "Installation and rollback failed, recover the .previous directory manually"
      ${EndIf}
    ${EndIf}
    Abort "Installation failed, previous native files were restored"
  ${EndIf}
  SetOutPath "$INSTDIR"
  ClearErrors
  WriteUninstaller "$INSTDIR\uninstall.exe"
  ${If} ${Errors}
    Abort "Cannot write the uninstaller, previous files remain in .previous for recovery"
  ${EndIf}
  nsExec::ExecToStack '"$INSTDIR\XiaoBaiSwitch.exe" --native-smoke'
  Pop $0
  Pop $1
  ${StrStr} $2 $1 '$\"version$\":$\"${VERSION}$\"'
  ${If} $0 != 0
  ${OrIf} $2 == ""
    Abort "Native executable verification failed, previous installation has not been removed"
  ${EndIf}
  ; Only remove MSI after a complete native executable is present and verified.
  ${If} $MsiProduct != ""
    ExecWait '"$SYSDIR\msiexec.exe" /x $MsiProduct /passive /norestart' $0
    ${If} $0 != 0
    ${AndIf} $0 != 3010
      Abort "Native files are installed, legacy MSI removal failed; both versions remain available"
    ${EndIf}
  ${EndIf}
  ClearErrors
  WriteRegStr HKCU "${MANU}" "" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL}" "" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL}" "DisplayName" "${PRODUCT}"
  WriteRegStr HKCU "${UNINSTALL}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTALL}" "Publisher" "github"
  WriteRegStr HKCU "${UNINSTALL}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL}" "UninstallString" '$"$INSTDIR\uninstall.exe$"'
  WriteRegDWORD HKCU "${UNINSTALL}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL}" "NoRepair" 1
  WriteRegStr HKCU "Software\Classes\${PROTOCOL}" "" "URL:XiaoBaiSwitch"
  WriteRegStr HKCU "Software\Classes\${PROTOCOL}" "URL Protocol" ""
  WriteRegStr HKCU "Software\Classes\${PROTOCOL}\shell\open\command" "" '$"$INSTDIR\XiaoBaiSwitch.exe$" $"%1$"'
  CreateShortcut "${SHORTCUT}" "$INSTDIR\XiaoBaiSwitch.exe"
  ${If} ${Errors}
    Abort "Native files are installed but registration failed; previous files remain for recovery"
  ${EndIf}
  ${If} $HadPrevious == 1
    ClearErrors
    RMDir /r "$INSTDIR.previous"
    ${If} ${Errors}
      MessageBox MB_ICONEXCLAMATION "Installation completed, previous files could not be removed"
    ${EndIf}
  ${EndIf}
SectionEnd
Function Launch
  Exec '"$INSTDIR\XiaoBaiSwitch.exe" $LaunchArgs'
FunctionEnd
Function .onInstFailed
  SetErrorLevel 1
FunctionEnd
Function .onInstSuccess
  ${If} ${Silent}
  ${AndIf} $Restart == 1
    Call Launch
  ${EndIf}
FunctionEnd
Section Uninstall
  SetRegView 64
  Delete "$INSTDIR\XiaoBaiSwitch.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "${SHORTCUT}"
  DeleteRegKey HKCU "${UNINSTALL}"
  DeleteRegKey HKCU "${MANU}"
  ReadRegStr $0 HKCU "Software\Classes\${PROTOCOL}\shell\open\command" ""
  ${If} $0 == '$"$INSTDIR\XiaoBaiSwitch.exe$" $"%1$"'
    DeleteRegKey HKCU "Software\Classes\${PROTOCOL}"
  ${EndIf}
SectionEnd
