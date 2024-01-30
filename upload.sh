git add .
git commit -m "$(date '+%Y-%m-%d %H:%M:%S') 更新"
git pull
hugo --cleanDestinationDir --gc
git push origin
git push github