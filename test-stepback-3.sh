# Run a passing commit, then run this script, then wait for stepback to happen
perl -i -pe 's/(?<!# )(exit 1)/# $1/g' evergreen.yml

git commit --allow-empty -m "1"
git commit --allow-empty -m "2"

perl -i -pe 's/# (exit 1)/$1/g' evergreen.yml
git commit -am "3"
perl -i -pe 's/(?<!# )(exit 1)/# $1/g' evergreen.yml

git commit --allow-empty -m "4"
git commit --allow-empty -m "5"
git commit --allow-empty -m "6"
git commit --allow-empty -m "7"
git commit --allow-empty -m "8"
git commit --allow-empty -m "9"
git commit --allow-empty -m "10"
git commit --allow-empty -m "Latest commit"

git push