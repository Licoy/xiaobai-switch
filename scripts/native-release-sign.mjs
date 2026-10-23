// Keep the certificate password out of the NSIS script and its compiler command line.
import {spawnSync} from 'node:child_process'
import {fileURLToPath} from 'node:url'
export const signerCheck=`$ErrorActionPreference = 'Stop'
$sig = Get-AuthenticodeSignature -LiteralPath $env:NATIVE_SIGNED_FILE
if ($sig.Status -ne 'Valid' -or !$sig.SignerCertificate -or $sig.SignerCertificate.Thumbprint -ne $env:WINDOWS_SIGN_CERT_SHA1) { throw 'Authenticode signature or signer certificate mismatch' }`
export function signWindowsFile(file,env=process.env){
 if(!file||!env.WINDOWS_SIGN_CERT_FILE||!env.WINDOWS_SIGN_CERT_PASSWORD||!/^[A-Fa-f0-9]{40}$/.test(env.WINDOWS_SIGN_CERT_SHA1??''))throw new Error('Windows signing certificate, password and SHA1 required')
 const tool=env.SIGNTOOL??'signtool'
 for(const [command,args] of [
  [tool,['sign','/fd','SHA256','/f',env.WINDOWS_SIGN_CERT_FILE,'/p',env.WINDOWS_SIGN_CERT_PASSWORD,'/tr','http://timestamp.digicert.com','/td','SHA256',file]],
  [tool,['verify','/pa',file]],
  ['powershell',['-NoProfile','-NonInteractive','-Command',signerCheck]],
 ]){
  const result=spawnSync(command,args,{stdio:'inherit',env:{...env,NATIVE_SIGNED_FILE:file}})
  if(result.status!==0)throw new Error(`Windows signing or verification failed (${result.status??result.error?.code})`)
 }
}
if(process.argv[1]===fileURLToPath(import.meta.url))signWindowsFile(process.argv[2])
