package blsloader

//func TestLoadKeys_SingleBls_File(t *testing.T) {
//	tests := []struct {
//		cfg    Config
//		inputs []string
//
//		expOutSubs []string
//		expPubKeys []string
//		expErr     error
//	}{
//		{
//			// load the default pass file with file
//			cfg: Config{
//				BlsKeyFile:  &validTestKeys[0].keyFile,
//				PassSrcType: PassSrcFile,
//			},
//			inputs: []string{},
//
//			expOutSubs: []string{},
//			expPubKeys: []string{validTestKeys[0].publicKey},
//		},
//		{
//			// load the default pass file with file
//			cfg: Config{
//				BlsKeyFile:  &validTestKeys[1].keyFile,
//				PassSrcType: PassSrcFile,
//			},
//			inputs: []string{},
//
//			expOutSubs: []string{},
//			expPubKeys: []string{validTestKeys[1].publicKey},
//		},
//		{
//			// load key file with prompt
//			cfg: Config{
//				BlsKeyFile:  &validTestKeys[1].keyFile,
//				PassSrcType: PassSrcPrompt,
//			},
//			inputs: []string{validTestKeys[1].passphrase},
//
//			expOutSubs: []string{
//				fmt.Sprintf("Enter passphrase for the BLS key file %s:", validTestKeys[1].keyFile),
//			},
//			expPubKeys: []string{validTestKeys[1].publicKey},
//		},
//		{
//			// Automatically use pass file
//			cfg: Config{
//				BlsKeyFile:  &validTestKeys[1].keyFile,
//				PassSrcType: PassSrcAuto,
//			},
//			inputs: []string{},
//
//			expOutSubs: []string{},
//			expPubKeys: []string{validTestKeys[1].publicKey},
//		},
//		{
//			// Automatically use prompt
//			cfg: Config{
//				BlsKeyFile:  &emptyPassTestKeys[1].keyFile,
//				PassSrcType: PassSrcAuto,
//			},
//			inputs: []string{emptyPassTestKeys[1].passphrase},
//
//			expOutSubs: []string{
//				"unable to get passphrase",
//				fmt.Sprintf("Enter passphrase for the BLS key file %s:", emptyPassTestKeys[1].keyFile),
//			},
//			expPubKeys: []string{emptyPassTestKeys[1].publicKey},
//		},
//	}
//	for i, test := range tests {
//		ts := &testSuite{
//			cfg:        test.cfg,
//			inputs:     test.inputs,
//			expOutSub:  test.expOutSubs,
//			expErr:     test.expErr,
//			expPubKeys: test.expPubKeys,
//		}
//		ts.init()
//		ts.process()
//		fmt.Println(111)
//		if err := ts.checkResult(); err != nil {
//			t.Errorf("test %v: %v", i, err)
//		}
//	}
//}
//

//}
//
//type testSuite struct {
//	cfg    Config
//	inputs []string
//
//	expOutSub  []string
//	expPubKeys []string
//	expErr     error
//
//	gotKeys    multibls.PrivateKeys
//	gotOutputs []string
//
//	timeout time.Duration
//	console *testConsole
//	gotErr  error // err returned from load key
//	errC    chan error
//	wg      sync.WaitGroup
//}
//
//func (ts *testSuite) init() {
//	ts.gotOutputs = make([]string, 0, len(ts.expOutSub))
//	ts.console = newTestConsole()
//	setTestConsole(ts.console)
//	ts.timeout = 1 * time.Second
//	ts.errC = make(chan error, 3)
//}
//
//func (ts *testSuite) process() {
//	ts.wg.Add(3)
//	go ts.threadedLoadOutputs()
//	go ts.threadedFeedConsoleInputs()
//	go ts.threadLoadKeys()
//
//	ts.wg.Wait()
//}
//
//func (ts *testSuite) checkResult() error {
//	if err := assertError(ts.gotErr, ts.expErr); err != nil {
//		return err
//	}
//	select {
//	case err := <-ts.errC:
//		return err
//	default:
//	}
//	fmt.Println("got outputs:", ts.gotOutputs)
//	fmt.Println("expect outputs:", ts.expOutSub)
//	if isClean, msg := ts.console.checkClean(); !isClean {
//		return fmt.Errorf("console not clean: %v", msg)
//	}
//	if ts.expErr != nil {
//		return nil
//	}
//	if err := ts.checkKeys(); err != nil {
//		return err
//	}
//	if err := ts.checkOutputs(); err != nil {
//		return err
//	}
//	return nil
//}
//
//func (ts *testSuite) checkOutputs() error {
//	if len(ts.gotOutputs) != len(ts.expOutSub) {
//		return fmt.Errorf("output size not expected: %v / %v", len(ts.gotOutputs), len(ts.expOutSub))
//	}
//	for i, gotOutput := range ts.gotOutputs {
//		expOutput := ts.expOutSub[i]
//		if !strings.Contains(gotOutput, expOutput) {
//			return fmt.Errorf("%vth output unexpected: [%v] / [%v]", i, gotOutput, expOutput)
//		}
//	}
//	return nil
//}
//
//func (ts *testSuite) checkKeys() error {
//	if len(ts.expPubKeys) != len(ts.gotKeys) {
//		return fmt.Errorf("loaded key size not expected: %v / %v", len(ts.gotKeys), len(ts.expPubKeys))
//	}
//	expKeyMap := make(map[bls.SerializedPublicKey]struct{})
//	for _, pubKeyStr := range ts.expPubKeys {
//		pubKey := pubStrToPubBytes(pubKeyStr)
//		if _, exist := expKeyMap[pubKey]; exist {
//			return fmt.Errorf("duplicate expect pubkey %x", pubKey)
//		}
//		expKeyMap[pubKey] = struct{}{}
//	}
//	gotVisited := make(map[bls.SerializedPublicKey]struct{})
//	for _, gotPubWrapper := range ts.gotKeys.GetPublicKeys() {
//		pubKey := gotPubWrapper.Bytes
//		if _, exist := gotVisited[pubKey]; exist {
//			return fmt.Errorf("duplicate got pubkey %x", pubKey)
//		}
//		if _, exist := expKeyMap[pubKey]; !exist {
//			return fmt.Errorf("got pubkey not found in expect %x", pubKey)
//		}
//	}
//	return nil
//}
//
//func (ts *testSuite) threadLoadKeys() {
//	defer ts.wg.Done()
//
//	ts.gotKeys, ts.gotErr = LoadKeys(ts.cfg)
//	return
//}
//
//func (ts *testSuite) threadedFeedConsoleInputs() {
//	defer ts.wg.Done()
//
//	i := 0
//	for i < len(ts.inputs) {
//		select {
//		case ts.console.In <- ts.inputs[i]:
//			i += 1
//		case <-time.After(ts.timeout):
//			ts.errC <- errors.New("feed inputs timed out")
//			return
//		}
//	}
//}
//
//func (ts *testSuite) threadedLoadOutputs() {
//	defer ts.wg.Done()
//	var (
//		i = 0
//	)
//	for i < len(ts.expOutSub) {
//		select {
//		case got := <-ts.console.Out:
//			ts.gotOutputs = append(ts.gotOutputs, got)
//			i++
//		case <-time.After(ts.timeout):
//			ts.errC <- errors.New("load outputs timed out")
//			return
//		}
//	}
//}
//
//func pubStrToPubBytes(str string) bls.SerializedPublicKey {
//	b := common.Hex2Bytes(str)
//	var pubKey bls.SerializedPublicKey
//	copy(pubKey[:], b)
//	return pubKey
//}
