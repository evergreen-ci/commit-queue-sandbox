perl -i -pe 's/echo 1/exit 1/g' generator.json # fail 1
git commit -am "1"
git commit --allow-empty -m "2"
git commit --allow-empty -m "3"
git commit --allow-empty -m "4"
git commit --allow-empty -m "5"
git commit --allow-empty -m "6"
git commit --allow-empty -m "7"
perl -i -pe 's/echo 2/exit 2/g' generator.json # fail 1
git commit -am "8"
git commit --allow-empty -m "9"
git commit --allow-empty -m "10"

git commit --allow-empty -m "Latest commit"

# Reset the file to its original state
perl -i -pe 's/exit 2/echo 2/g' generator.json # fail 1
perl -i -pe 's/exit 2/echo 2/g' generator.json # fail 1

git push
