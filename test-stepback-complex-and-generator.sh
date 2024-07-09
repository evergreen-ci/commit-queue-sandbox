perl -i -pe 's/# (exit 1 #1)/$1/g' evergreen.yml # fail 1
git commit -am "1"
perl -i -pe 's/# (exit 1 #2)/$1/g' evergreen.yml # fail 2
git commit -am "2"
perl -i -pe 's/echo 1/exit 1/g' generator.json # generator fail 1
git commit -am "3"
git commit --allow-empty -m "4"
perl -i -pe 's/# (exit 1 #5)/$1/g' evergreen.yml # fail 5
git commit -am "5"
git commit --allow-empty -m "6"
perl -i -pe 's/# (exit 1 #7)/$1/g' evergreen.yml # fail 7
git commit -am "7"
perl -i -pe 's/# (exit 1 #8)/$1/g' evergreen.yml # fail 8
git commit -am "8"
git commit --allow-empty -m "9"
perl -i -pe 's/echo 2/exit 2/g' generator.json # generator fail 2
git commit --allow-empty -m "10"

git commit --allow-empty -m "Latest commit"

# Reset the evergreen config to its original state
perl -i -pe 's/(?<!# )(exit 1 #1)/# $1/g' evergreen.yml #fail 1
perl -i -pe 's/(?<!# )(exit 1 #2)/# $1/g' evergreen.yml #fail 2
perl -i -pe 's/(?<!# )(exit 1 #5)/# $1/g' evergreen.yml #fail 5
perl -i -pe 's/(?<!# )(exit 1 #7)/# $1/g' evergreen.yml #fail 7
perl -i -pe 's/(?<!# )(exit 1 #8)/# $1/g' evergreen.yml #fail 8
# Reset the generator config to its original state
perl -i -pe 's/exit 1/echo 1/g' generator.json # generator fail 1
perl -i -pe 's/exit 2/echo 2/g' generator.json # generator fail 2

git push
