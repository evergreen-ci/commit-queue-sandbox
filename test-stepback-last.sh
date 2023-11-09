# Run a passing commit, then run this script, then wait for stepback to happen
perl -i -pe 's/(?<!# )(exit 1)/# $1/g' evergreen.yml

git commit --allow-empty -m "1"
git commit --allow-empty -m "2"
git commit --allow-empty -m "3"
git commit --allow-empty -m "4"
git commit --allow-empty -m "5"
git commit --allow-empty -m "6"
git commit --allow-empty -m "7"
git commit --allow-empty -m "8"
git commit --allow-empty -m "9"
git commit --allow-empty -m "10"

perl -i -pe 's/# (exit 1)/$1/g' evergreen.yml
git commit -am "Latest commit"
perl -i -pe 's/(?<!# )(exit 1)/# $1/g' evergreen.yml

git push
