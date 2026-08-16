# go-unittest-gen-sample

UnitTest を自動生成する Agent の動作確認用 Go リポジトリです。

`checkout.Calculate` は、テスト生成の題材になるように次の性質を持っています。

- 正常系と複数の入力エラー
- 会員ランク、送料無料ライン、離島料金による分岐
- パーセント／固定額クーポンと適用条件
- 割引と税の端数処理、および割引額の上限
- `CouponStore` のテストダブルと `context.Context` の受け渡し
- `errors.Is` で検査できる sentinel error

Agent に対して、たとえば次のように依頼できます。

> `checkout.Calculate` の仕様と分岐を網羅するテーブル駆動 UnitTest を生成してください。
