#ifndef AppVersion
  #define AppVersion "0.1.0-dev"
#endif
#ifndef NumericVersion
  #define NumericVersion "0.1.0"
#endif
#define AppExe "kuba-equalizer.exe"
#define Bootstrapper "MicrosoftEdgeWebview2Setup.exe"

[Setup]
AppId=audio.kuba.equalizer
AppName=Equalizador Kuba
AppVersion={#AppVersion}
AppPublisher=João Vitor
AppPublisherURL=https://devjvao.com/
AppUpdatesURL=https://github.com/devjvao/kuba-equalizer/releases
AppMutex=wails-app-audio.kuba.equalizer-sim
DefaultDirName={localappdata}\Programs\Equalizador Kuba
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
OutputBaseFilename=kuba-equalizer-setup-windows-amd64
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
Source: "..\build\bin\kuba-equalizer-windows-amd64.exe"; DestDir: "{app}"; DestName: "{#AppExe}"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\Equalizador Kuba"; Filename: "{app}\{#AppExe}"; WorkingDir: "{app}"

[Run]
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,Equalizador Kuba}"; Flags: postinstall nowait skipifsilent unchecked

[Code]
const
  WebView2Key = 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  StartupKey = 'Software\Microsoft\Windows\CurrentVersion\Run';
  StartupName = 'Kuba Equalizer';
  StartupArg = ' --windows-startup';

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

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Command, InstalledExe: String;
begin
  if CurUninstallStep <> usPostUninstall then
    Exit;

  InstalledExe := ExpandConstant('{app}\{#AppExe}');
  if RegQueryStringValue(HKCU, StartupKey, StartupName, Command) and
    (SameText(Command, '"' + InstalledExe + '"' + StartupArg) or
     SameText(Command, InstalledExe + StartupArg)) then
    RegDeleteValue(HKCU, StartupKey, StartupName);
end;
