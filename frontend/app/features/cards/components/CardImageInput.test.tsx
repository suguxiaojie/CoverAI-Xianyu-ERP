// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen } from '@testing-library/react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import { CardImageInput } from './CardImageInput';

describe('CardImageInput 图片来源输入', /* 当前回调验证拖拽、替换、校验和 URL 兼容入口。 */ () => {
  beforeEach(/* 当前回调安装临时对象 URL 替身，避免 jsdom 缺少浏览器文件预览能力。 */ () => {
    vi.stubGlobal('URL', {
      ...URL,
      createObjectURL: vi.fn().mockReturnValue('blob:card-image'),
      revokeObjectURL: vi.fn(),
    });
  });

  afterEach(/* 当前回调清理组件 DOM 和全局对象 URL 替身。 */ () => {
    cleanup();
    vi.unstubAllGlobals();
  });

  test('拖入合法图片并显示文件预览信息', /* 当前回调验证图片文件只进入受控表单状态。 */ () => {
    // onFileChange 是父表单接收拖入文件的替身。
    const onFileChange = vi.fn();
    // image 是符合客户端类型和体积门禁的 PNG 文件。
    const image = new File(['png'], '教程图.png', { type: 'image/png' });
    // view 保存组件渲染结果，用于模拟父表单接收文件后的受控重渲染。
    const view = render(
      <CardImageInput mode="upload" file={null} remoteURL="" onModeChange={vi.fn()} onFileChange={onFileChange} onRemoteURLChange={vi.fn()} />,
    );
    fireEvent.drop(screen.getByText('拖动图片到这里，或点击选择').closest('label') as HTMLLabelElement, { dataTransfer: { files: [image] } });
    expect(onFileChange).toHaveBeenCalledWith(image);
    view.rerender(<CardImageInput mode="upload" file={image} remoteURL="" onModeChange={vi.fn()} onFileChange={onFileChange} onRemoteURLChange={vi.fn()} />);
    expect(screen.getByText('教程图.png')).toBeTruthy();
    expect(screen.getByAltText('卡密图片预览').getAttribute('src')).toBe('blob:card-image');
  });

  test('拒绝非图片并可切换到 URL 兼容入口', /* 当前回调验证客户端即时错误和高级入口。 */ () => {
	// onModeChange 记录上传和远程 URL 来源切换动作。
    const onModeChange = vi.fn();
	// onFileChange 记录非法文件清空或合法文件选择动作。
    const onFileChange = vi.fn();
    render(<CardImageInput mode="upload" file={null} remoteURL="" onModeChange={onModeChange} onFileChange={onFileChange} onRemoteURLChange={vi.fn()} />);
    // fakeImage 是伪装文件名但 MIME 明确为文本的非法输入。
    const fakeImage = new File(['text'], 'fake.png', { type: 'text/plain' });
    fireEvent.drop(screen.getByText('拖动图片到这里，或点击选择').closest('label') as HTMLLabelElement, { dataTransfer: { files: [fakeImage] } });
    expect(screen.getByRole('alert').textContent).toContain('只支持 PNG');
    expect(onFileChange).toHaveBeenCalledWith(null);
    fireEvent.click(screen.getByText('高级：使用图片 URL'));
    expect(onModeChange).toHaveBeenCalledWith('url');
  });

  test('URL 模式保留远程地址输入和预览失败提示', /* 当前回调验证旧图片 URL 流程仍可使用。 */ () => {
    // onRemoteURLChange 是父表单接收 URL 文本变化的替身。
    const onRemoteURLChange = vi.fn();
    render(<CardImageInput mode="url" file={null} remoteURL="https://example.com/card.png" onModeChange={vi.fn()} onFileChange={vi.fn()} onRemoteURLChange={onRemoteURLChange} />);
    fireEvent.change(screen.getByPlaceholderText('https://example.com/card.png'), { target: { value: 'https://cdn.example/new.png' } });
    expect(onRemoteURLChange).toHaveBeenCalledWith('https://cdn.example/new.png');
    fireEvent.error(screen.getByAltText('卡密图片预览'));
    expect(screen.getByRole('alert').textContent).toContain('图片预览加载失败');
    expect(screen.getByText('改用拖拽上传')).toBeTruthy();
  });
});
