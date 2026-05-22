; Vex Agent Windows Installer
; Build: makensis -DVERSION=x.y.z -DBIN_PATH=<path-to-vex-agent.exe> vex-agent.nsi
; Requires: makensis (apt install nsis)

!include "MUI2.nsh"

!define PRODUCT_NAME "Vex Agent"
!define PRODUCT_PUBLISHER "Jimber Software"
!define SERVICE_NAME "VexAgent"
!define INSTALL_DIR "$PROGRAMFILES\Vex"
!define UNINSTALL_REG_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${SERVICE_NAME}"

!ifndef VERSION
  !define VERSION "0.0.0-dev"
!endif

Name "${PRODUCT_NAME} ${VERSION}"
OutFile "../../dist/vex-agent-installer.exe"
InstallDir "${INSTALL_DIR}"
RequestExecutionLevel admin
SetCompressor /SOLID lzma

!define MUI_ABORTWARNING
!define MUI_ICON "${NSISDIR}\Contrib\Graphics\Icons\modern-install.ico"
!define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\modern-uninstall.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Section "Install"
    SetOutPath "${INSTALL_DIR}"

    ; Stop existing service if upgrading
    nsExec::ExecToLog 'sc stop ${SERVICE_NAME}'
    nsExec::ExecToLog 'sc delete ${SERVICE_NAME}'
    Sleep 2000

    ; Install binary
    File "${BIN_PATH}"

    ; Create the service
    nsExec::ExecToLog 'sc create ${SERVICE_NAME} binPath= "${INSTALL_DIR}\vex-agent.exe" start= auto'
    nsExec::ExecToLog 'sc description ${SERVICE_NAME} "Vex guest agent - vsock listener for host-to-guest communication"'
    nsExec::ExecToLog 'sc failure ${SERVICE_NAME} reset= 86400 actions= restart/5000/restart/10000/restart/30000'

    ; Start the service
    nsExec::ExecToLog 'sc start ${SERVICE_NAME}'

    ; Write uninstaller
    WriteUninstaller "${INSTALL_DIR}\uninstall.exe"

    ; Registry entries for Add/Remove Programs
    WriteRegStr HKLM "${UNINSTALL_REG_KEY}" "DisplayName" "${PRODUCT_NAME}"
    WriteRegStr HKLM "${UNINSTALL_REG_KEY}" "DisplayVersion" "${VERSION}"
    WriteRegStr HKLM "${UNINSTALL_REG_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
    WriteRegStr HKLM "${UNINSTALL_REG_KEY}" "UninstallString" "${INSTALL_DIR}\uninstall.exe"
    WriteRegStr HKLM "${UNINSTALL_REG_KEY}" "InstallLocation" "${INSTALL_DIR}"
    WriteRegDWORD HKLM "${UNINSTALL_REG_KEY}" "NoModify" 1
    WriteRegDWORD HKLM "${UNINSTALL_REG_KEY}" "NoRepair" 1
SectionEnd

Section "Uninstall"
    ; Stop and remove service
    nsExec::ExecToLog 'sc stop ${SERVICE_NAME}'
    Sleep 2000
    nsExec::ExecToLog 'sc delete ${SERVICE_NAME}'
    Sleep 1000

    ; Remove files
    Delete "${INSTALL_DIR}\vex-agent.exe"
    Delete "${INSTALL_DIR}\uninstall.exe"
    RMDir "${INSTALL_DIR}"

    ; Remove registry
    DeleteRegKey HKLM "${UNINSTALL_REG_KEY}"
SectionEnd
