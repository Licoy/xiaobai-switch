import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {generateKeyPairSync} from 'node:crypto'
import {preflight,macSigningConfig} from './native-release.mjs'
import {assertUnsignedDMG} from './native-release-package.mjs'
import {signWindowsFile,signerCheck} from './native-release-sign.mjs'
function credentials(){
 const {privateKey,publicKey}=generateKeyPairSync('ed25519')
 const pub=publicKey.export({type:'spki',format:'der'}).subarray(-32)
 const seed=privateKey.export({type:'pkcs8',format:'der'}).subarray(-32)
 return {RELEASE_VERSION:'1.0.0',RELEASE_NOTES:'Test release',MYGO_UPDATER_PUBLIC_KEY:pub.toString('base64'),MYGO_UPDATER_PRIVATE_KEY:Buffer.concat([seed,pub]).toString('base64'),TAURI_SIGNING_PRIVATE_KEY:'untrusted comment: test fixture',TAURI_SIGNING_PRIVATE_KEY_PASSWORD:'fixture',APPLE_SIGNING_IDENTITY:'Fixture Self-Signed',APPLE_SIGNING_CERT_SHA1:'A'.repeat(40),WINDOWS_SIGN_CERT_FILE:'fixture.pfx',WINDOWS_SIGN_CERT_PASSWORD:'fixture',WINDOWS_SIGN_CERT_SHA1:'B'.repeat(40)}
}
test('self-signed preflight requires certificate binding without Apple account or notary configuration',()=>{
 const env=credentials()
 for(const target of ['darwin-amd64','darwin-arm64','windows-amd64','windows-arm64'])assert(preflight(env,target))
 assert.deepEqual(macSigningConfig(env),{minimumSystemVersion:'12.0',signingIdentity:'Fixture Self-Signed'})
 for(const name of ['APPLE_SIGNING_IDENTITY','APPLE_SIGNING_CERT_SHA1'])assert.throws(()=>preflight({...env,[name]:''},'darwin-arm64'),/missing/)
 for(const name of ['WINDOWS_SIGN_CERT_FILE','WINDOWS_SIGN_CERT_PASSWORD','WINDOWS_SIGN_CERT_SHA1'])assert.throws(()=>preflight({...env,[name]:''},'windows-amd64'),/missing/)
 assert.throws(()=>preflight({...env,APPLE_SIGNING_IDENTITY:'-'},'darwin-arm64'),/ad hoc/)
 assert.throws(()=>preflight({...env,APPLE_SIGNING_CERT_SHA1:'invalid'},'darwin-arm64'),/SHA1/)
 assert.throws(()=>preflight({...env,WINDOWS_SIGN_CERT_SHA1:'invalid'},'windows-arm64'),/SHA1/)
 assert.throws(()=>signWindowsFile('fixture.exe',{WINDOWS_SIGN_CERT_FILE:'fixture.pfx',WINDOWS_SIGN_CERT_PASSWORD:'fixture'}),/SHA1/)
})
test('unsigned DMG confirmation rejects retained signatures and unexpected verification errors',()=>{
 assert.doesNotThrow(()=>assertUnsignedDMG({status:1,stdout:'',stderr:'fixture.dmg: code object is not signed at all\n'}))
 for(const result of [{status:0,stdout:'',stderr:''},{status:1,stdout:'',stderr:'file not found'},{status:null,error:Error('spawn failed'),stderr:'code object is not signed at all'}])assert.throws(()=>assertUnsignedDMG(result),/not confirmed unsigned/)
})
test('CI and all packaging paths keep signature checks and temporary trust cleanup mandatory',()=>{
 const workflow=fs.readFileSync(new URL('../.github/workflows/native-release.yml',import.meta.url),'utf8')
 const pack=fs.readFileSync(new URL('./native-release-package.mjs',import.meta.url),'utf8')
 const sign=fs.readFileSync(new URL('./native-release-sign.mjs',import.meta.url),'utf8')
 assert.doesNotMatch(workflow,/APPLE_ID|APPLE_PASSWORD|APPLE_TEAM_ID|NOTARY|notarytool|LocalMachine/)
 for(const token of ['sudo security add-trusted-cert -d -r trustRoot -p codeSign','sudo security remove-trusted-cert -d','security delete-keychain',"@('Root', 'TrustedPublisher')",'Remove-Item $cert','WINDOWS_SIGN_CERT_SHA1='])assert(workflow.includes(token),token)
 assert(pack.includes('signWindowsFile(f,env)'))
 assert(pack.includes("'--verify','--deep','--strict','-R',requirement"))
 assert(pack.includes("'--verify','--strict','-R',requirement"))
 assert(sign.includes("[tool,['verify','/pa',file]]"))
 assert(signerCheck.includes("$sig.Status -ne 'Valid'"))
 assert(signerCheck.includes('$sig.SignerCertificate.Thumbprint -ne $env:WINDOWS_SIGN_CERT_SHA1'))
})
