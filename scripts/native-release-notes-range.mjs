// Bound git-cliff to this tag and the nearest earlier release on its history.
import {execFileSync} from 'node:child_process'
import {fileURLToPath} from 'node:url'
import {version} from './native-release-lib.mjs'
export function notesRange(v,cwd=process.cwd()) {
 version(v)
 const git=(...args)=>execFileSync('git',args,{cwd,encoding:'utf8'}).trim()
 const current=git('rev-parse',`refs/tags/v${v}^{commit}`)
 const tags=git('tag','--merged',current).split('\n').filter(tag=>/^v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/.test(tag))
 const prior=[]
 for(const tag of tags) {
  if(git('rev-parse',`refs/tags/${tag}^{commit}`)!==current&&(v.includes('-')||!tag.includes('-')))prior.push(tag)
 }
 if(!prior.length)return current
 const previous=git('describe','--tags','--abbrev=0',...prior.flatMap(tag=>['--match',tag]),current)
 return `${git('rev-parse',`refs/tags/${previous}^{commit}`)}..${current}`
}
if(process.argv[1]===fileURLToPath(import.meta.url))console.log(notesRange(process.env.RELEASE_VERSION))
