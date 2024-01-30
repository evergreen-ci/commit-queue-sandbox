perl -i -pe 's/# (exit 1 #1)/$1/g' evergreen.yml # fail 1
git commit -m "1"
perl -i -pe 's/# (exit 1 #2)/$1/g' evergreen.yml # fail 2
git commit -m "2"
git commit --allow-empty -m "3"
git commit --allow-empty -m "4"
perl -i -pe 's/# (exit 1 #5)/$1/g' evergreen.yml # fail 5
git commit -m "5"
git commit --allow-empty -m "6"
perl -i -pe 's/# (exit 1 #7)/$1/g' evergreen.yml # fail 7
git commit --allow-empty -m "7"
perl -i -pe 's/# (exit 1 #8)/$1/g' evergreen.yml # fail 8
git commit --allow-empty -m "8"
git commit --allow-empty -m "9"
git commit --allow-empty -m "10"

git commit -am "Latest commit"

# Reset the file to its original state
perl -i -pe 's/(?<!# )(exit 1 #1)/# $1/g' evergreen.yml #fail 1
perl -i -pe 's/(?<!# )(exit 1 #2)/# $1/g' evergreen.yml #fail 2
perl -i -pe 's/(?<!# )(exit 1 #5)/# $1/g' evergreen.yml #fail 5
perl -i -pe 's/(?<!# )(exit 1 #7)/# $1/g' evergreen.yml #fail 7
perl -i -pe 's/(?<!# )(exit 1 #8)/# $1/g' evergreen.yml #fail 8

git push
