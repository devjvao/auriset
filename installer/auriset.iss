#ifndef AppVersion
  #define AppVersion "0.1.0-dev"
#endif
#ifndef NumericVersion
  #define NumericVersion "0.1.0"
#endif
#define AppExe "auriset.exe"
#define Bootstrapper "MicrosoftEdgeWebview2Setup.exe"

[Setup]
AppId=audio.kuba.equalizer
AppName=Auriset
AppVersion={#AppVersion}
AppComments=App não oficial para trocar os presets de equalização do cabo USB-C Kuba Auri no Windows.
AppPublisher=João Vitor
AppPublisherURL=https://devjvao.com/
AppUpdatesURL=https://github.com/devjvao/auriset/releases
AppMutex=wails-app-audio.kuba.equalizer-sim
DefaultDirName={localappdata}\Programs\Auriset
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
MinVersion=10.0
DisableProgramGroupPage=yes
ShowLanguageDialog=no
RestartApplications=no
UninstallDisplayIcon={app}\{#AppExe}
VersionInfoVersion={#NumericVersion}
VersionInfoTextVersion={#AppVersion}
OutputDir=..\build\bin
OutputBaseFilename=auriset-setup-windows-amd64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Messages]
SetupAppRunningError=%1 está aberto. Use a opção Sair no ícone da área de notificação, perto do relógio, e clique em OK para continuar.
UninstallAppRunningError=%1 está aberto. Use a opção Sair no ícone da área de notificação, perto do relógio, e clique em OK para continuar.

[CustomMessages]
WebView2InstallError=Não foi possível instalar o Microsoft Edge WebView2 Runtime. Verifique a conexão com a Internet ou instale o runtime pelo site da Microsoft e tente novamente.

[Files]
Source: "..\build\bin\{#Bootstrapper}"; Flags: dontcopy
Source: "..\build\bin\auriset-windows-amd64.exe"; DestDir: "{app}"; DestName: "{#AppExe}"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\Auriset"; Filename: "{app}\{#AppExe}"; WorkingDir: "{app}"

[Run]
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,Auriset}"; Flags: postinstall nowait skipifsilent unchecked

[Code]
const
  WebView2Key = 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  StartupKey = 'Software\Microsoft\Windows\CurrentVersion\Run';
  StartupName = 'Kuba Equalizer';
  StartupArg = ' --windows-startup';

function StartupTargets(const Command, Exe: String): Boolean;
begin
  Result := SameText(Command, '"' + Exe + '"' + StartupArg) or
    SameText(Command, Exe + StartupArg);
end;

function WebView2VersionExists(RootKey: Integer): Boolean;
var
  Version: String;
begin
  Result := RegQueryStringValue(RootKey, WebView2Key, 'pv', Version) and
    (Version <> '') and (Version <> '0.0.0.0');
end;

function WebView2Installed: Boolean;
begin
  Result := WebView2VersionExists(HKLM32) or WebView2VersionExists(HKCU);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  ExitCode: Integer;
begin
  Result := '';
  if WebView2Installed then
    Exit;

  try
    ExtractTemporaryFile('{#Bootstrapper}');
    if Exec(ExpandConstant('{tmp}\{#Bootstrapper}'), '/silent /install', '', SW_HIDE,
      ewWaitUntilTerminated, ExitCode) then
      Log(Format('WebView2 bootstrapper exited with code %d.', [ExitCode]))
    else
      Log('Could not run the WebView2 bootstrapper: ' + SysErrorMessage(ExitCode));
  except
    Log(GetExceptionMessage);
  end;
  if not WebView2Installed then
    Result := CustomMessage('WebView2InstallError');
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Command, InstalledExe, OldExe, OldShortcut: String;
begin
  if CurStep <> ssPostInstall then
    Exit;

  InstalledExe := ExpandConstant('{app}\{#AppExe}');
  OldExe := ExpandConstant('{app}\kuba-equalizer.exe');
  if not FileExists(InstalledExe) or not FileExists(OldExe) then
    Exit;

  { Redirect only the startup entry for this installation; leave a portable copy alone. }
  if RegQueryStringValue(HKCU, StartupKey, StartupName, Command) and
    StartupTargets(Command, OldExe) and
    not RegWriteStringValue(HKCU, StartupKey, StartupName, '"' + InstalledExe + '"' + StartupArg) then
  begin
    Log('Could not migrate the Windows startup entry; retaining the previous executable.');
    Exit;
  end;

  if not DeleteFile(OldExe) then
    Log('Could not remove the previous executable: ' + OldExe);
  OldShortcut := ExpandConstant('{userprograms}\Equalizador Kuba.lnk');
  if FileExists(OldShortcut) and not DeleteFile(OldShortcut) then
    Log('Could not remove the old Start menu shortcut: ' + OldShortcut);
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Command, InstalledExe: String;
begin
  if CurUninstallStep <> usPostUninstall then
    Exit;

  InstalledExe := ExpandConstant('{app}\{#AppExe}');
  if RegQueryStringValue(HKCU, StartupKey, StartupName, Command) and
    StartupTargets(Command, InstalledExe) then
    RegDeleteValue(HKCU, StartupKey, StartupName);
end;
