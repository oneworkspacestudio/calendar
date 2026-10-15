param([ValidateSet('calendar','contacts','mail')][Parameter(Mandatory)][string]$App,[Parameter(Mandatory)][string]$Programs,[Parameter(Mandatory)][string]$SharedData)
$ErrorActionPreference='Stop'
$names=@{calendar='Calendar.exe';contacts='Contacts.exe';mail='Mail.exe'}
$appPath=Join-Path $Programs $names[$App]
Get-Process -Name ([IO.Path]::GetFileNameWithoutExtension($names[$App])) -ErrorAction SilentlyContinue | ForEach-Object {if($_.Path -eq $appPath){throw 'Close this app before uninstalling it. Other apps can stay open.'}}
& (Join-Path $Programs 'WorkspaceServices.exe') uninstall-app --root $SharedData --app $App
if($LASTEXITCODE){throw 'Shared app registration was not updated; no program was removed'}
if(Test-Path -LiteralPath $appPath){Remove-Item -LiteralPath $appPath}
Write-Output 'Removed this app executable/registration. Shared service, other apps and all user data were retained.'
